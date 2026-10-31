//go:build unit

package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type clientReleaseMemoryRepo struct {
	releases   map[int64]*ClientRelease
	next       int64
	saveErr    error
	publishErr error
}

func (r *clientReleaseMemoryRepo) List(ctx context.Context) ([]*ClientRelease, error) {
	out := make([]*ClientRelease, 0, len(r.releases))
	for id := range r.releases {
		release, _ := r.GetByID(ctx, id)
		out = append(out, release)
	}
	return out, nil
}
func (r *clientReleaseMemoryRepo) GetByID(_ context.Context, id int64) (*ClientRelease, error) {
	release, ok := r.releases[id]
	if !ok {
		return nil, ErrClientReleaseNotFound
	}
	clone := *release
	clone.Files = append([]ClientReleaseFile{}, release.Files...)
	return &clone, nil
}
func (r *clientReleaseMemoryRepo) GetByVersion(ctx context.Context, v string) (*ClientRelease, error) {
	for id, release := range r.releases {
		if release.Version == v {
			return r.GetByID(ctx, id)
		}
	}
	return nil, ErrClientReleaseNotFound
}
func (r *clientReleaseMemoryRepo) Create(ctx context.Context, v string, m json.RawMessage) (*ClientRelease, error) {
	for _, release := range r.releases {
		if release.Version == v {
			return nil, ErrClientReleaseVersionExists
		}
	}
	r.next++
	r.releases[r.next] = &ClientRelease{ID: r.next, Product: "qxapi", Version: v, Manifest: m, Status: ClientReleaseDraft, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	return r.GetByID(ctx, r.next)
}
func (r *clientReleaseMemoryRepo) SaveFile(_ context.Context, id int64, f ClientReleaseFile) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	release := r.releases[id]
	for i, old := range release.Files {
		if old.Name == f.Name {
			release.Files[i] = f
			return nil
		}
	}
	release.Files = append(release.Files, f)
	return nil
}
func (r *clientReleaseMemoryRepo) UpdateNotes(_ context.Context, id int64, notes string) error {
	release, ok := r.releases[id]
	if !ok {
		return ErrClientReleaseNotFound
	}
	release.Notes = notes
	return nil
}
func (r *clientReleaseMemoryRepo) Publish(_ context.Context, id int64, removed []int64) error {
	if r.publishErr != nil {
		return r.publishErr
	}
	r.releases[id].Status = ClientReleasePublished
	now := time.Now()
	r.releases[id].PublishedAt = &now
	for _, old := range removed {
		delete(r.releases, old)
	}
	return nil
}
func (r *clientReleaseMemoryRepo) Withdraw(_ context.Context, id int64) error {
	r.releases[id].Status = ClientReleaseWithdrawn
	return nil
}
func (r *clientReleaseMemoryRepo) Delete(_ context.Context, id int64) error {
	delete(r.releases, id)
	return nil
}

type clientReleaseSettingRepo struct{ SettingRepository }

func (clientReleaseSettingRepo) GetValue(context.Context, string) (string, error) {
	return "https://panel.example/", nil
}

func newClientReleaseTestService(t *testing.T) (*ClientReleaseService, *clientReleaseMemoryRepo) {
	t.Helper()
	t.Setenv("DATA_DIR", t.TempDir())
	repo := &clientReleaseMemoryRepo{releases: make(map[int64]*ClientRelease)}
	settings := &SettingService{settingRepo: clientReleaseSettingRepo{}, cfg: &config.Config{}}
	return NewClientReleaseService(repo, settings), repo
}

const clientTestFile = "qxapi-windows-amd64.exe"

func clientTestManifest(version string, data []byte) json.RawMessage {
	m, _ := json.Marshal(ClientReleaseManifest{Version: version, Files: map[string]ClientReleaseFileSpec{clientTestFile: {Size: int64(len(data)), SHA256: fmt.Sprintf("%x", sha256.Sum256(data))}}})
	return m
}
func TestClientReleaseManifest(t *testing.T) {
	valid := clientTestManifest("0.1.0", []byte("hello"))
	for _, tc := range []struct {
		name string
		raw  []byte
		want error
	}{
		{"valid", valid, nil},
		{"invalid JSON", []byte("{"), ErrClientReleaseInvalidManifest},
		{"trailing JSON", append(append([]byte{}, valid...), []byte("{}")...), ErrClientReleaseInvalidManifest},
		{"empty files", []byte(`{"version":"1.0.0","files":{}}`), ErrClientReleaseInvalidManifest},
		{"null", []byte(`null`), ErrClientReleaseInvalidVersion},
		{"invalid version", clientTestManifest("dev", []byte("x")), ErrClientReleaseInvalidVersion},
		{"metadata", clientTestManifest("1.0.0+build", []byte("x")), ErrClientReleaseInvalidVersion},
		{"dirty", clientTestManifest("1.0.0-dirty", []byte("x")), ErrClientReleaseInvalidVersion},
		{"describe", clientTestManifest("v0.1.0-3-g0bcb2cc", []byte("x")), ErrClientReleaseInvalidVersion},
		{"traversal version", clientTestManifest("1.0.0-../../etc", []byte("x")), ErrClientReleaseInvalidVersion},
		{"traversal file", bytes.Replace(valid, []byte(clientTestFile), []byte("../qxapi-windows-amd64.exe"), 1), ErrClientReleaseInvalidFileName},
		{"wrong platform", bytes.Replace(valid, []byte(clientTestFile), []byte("qxapi-freebsd-amd64"), 1), ErrClientReleaseInvalidFileName},
		{"zero size", bytes.Replace(valid, []byte(`"size":5`), []byte(`"size":0`), 1), ErrClientReleaseInvalidManifest},
		{"negative size", bytes.Replace(valid, []byte(`"size":5`), []byte(`"size":-1`), 1), ErrClientReleaseInvalidManifest},
		{"fractional size", bytes.Replace(valid, []byte(`"size":5`), []byte(`"size":1.5`), 1), ErrClientReleaseInvalidManifest},
		{"overflow size", bytes.Replace(valid, []byte(`"size":5`), []byte(`"size":9223372036854775808`), 1), ErrClientReleaseInvalidManifest},
		{"bad hash", bytes.Replace(valid, []byte(fmt.Sprintf("%x", sha256.Sum256([]byte("hello")))), []byte(strings.Repeat("G", 64)), 1), ErrClientReleaseInvalidManifest},
	} {
		t.Run(tc.name, func(t *testing.T) { _, err := ParseClientReleaseManifest(tc.raw); require.ErrorIs(t, err, tc.want) })
	}
	for _, version := range []string{"v1.2.3", " 1.2.3 ", "1.0.0-rc.2", "1.0.0-beta.10"} {
		_, err := ParseClientReleaseManifest(clientTestManifest(version, []byte("x")))
		require.NoError(t, err)
	}
	for _, name := range []string{"qxapi-windows-arm64.exe", "qxapi-darwin-arm64.zip", "qxapi-darwin-amd64.zip", "qxapi-macos-arm64.dmg", "qxapi-macos-amd64.dmg", "qxapi-linux-amd64", "qxapi-linux-arm64"} {
		_, err := ParseClientReleaseManifest(bytes.Replace(valid, []byte(clientTestFile), []byte(name), 1))
		require.NoError(t, err)
	}
}
func TestClientReleaseStorageRoot(t *testing.T) {
	t.Setenv("DATA_DIR", "")
	require.Equal(t, filepath.Join("data", "client-releases"), NewClientReleaseService(nil, nil).root)
	t.Setenv("DATA_DIR", "/custom/data")
	require.Equal(t, "/custom/data/client-releases", NewClientReleaseService(nil, nil).root)
}
func TestClientReleaseUpload(t *testing.T) {
	ctx := context.Background()
	data := []byte("hello")
	for _, tc := range []struct {
		name    string
		data    []byte
		bounded bool
	}{
		{"short", []byte("hell"), false}, {"long", []byte("hello!"), false}, {"sha256", []byte("HELLO"), false}, {"max bytes", []byte("hello!"), true}, {"valid", data, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, repo := newClientReleaseTestService(t)
			r, err := s.Create(ctx, clientTestManifest("v0.1.0", data))
			require.NoError(t, err)
			require.Equal(t, "0.1.0", r.Version)
			var body io.Reader = bytes.NewReader(tc.data)
			if tc.bounded {
				body = http.MaxBytesReader(httptest.NewRecorder(), io.NopCloser(body), int64(len(data)))
			}
			got, err := s.Upload(ctx, r.ID, clientTestFile, body)
			path := filepath.Join(s.releaseDir(r), clientTestFile)
			require.NoFileExists(t, path+".part")
			if tc.name != "valid" {
				require.ErrorIs(t, err, ErrClientReleaseFileMismatch)
				require.NoFileExists(t, path)
				require.Empty(t, repo.releases[r.ID].Files)
				return
			}
			require.NoError(t, err)
			require.Len(t, got.Files, 1)
			stored, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, data, stored)
			_, err = s.Upload(ctx, r.ID, clientTestFile, bytes.NewReader([]byte("HELLO")))
			require.ErrorIs(t, err, ErrClientReleaseFileMismatch)
			stored, err = os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, data, stored)
			_, err = s.Upload(ctx, r.ID, clientTestFile, bytes.NewReader(data))
			require.NoError(t, err)
			require.Len(t, repo.releases[r.ID].Files, 1)
		})
	}
}
func TestClientReleaseCheckUpload(t *testing.T) {
	s, repo := newClientReleaseTestService(t)
	ctx := context.Background()
	r, err := s.Create(ctx, clientTestManifest("1.0.0", []byte("hello")))
	require.NoError(t, err)
	for _, tc := range []struct {
		name, status, file string
		id                 int64
		want               error
	}{
		{"missing", ClientReleaseDraft, clientTestFile, 99, ErrClientReleaseNotFound},
		{"draft", ClientReleaseDraft, clientTestFile, r.ID, nil},
		{"withdrawn", ClientReleaseWithdrawn, clientTestFile, r.ID, nil},
		{"published", ClientReleasePublished, clientTestFile, r.ID, ErrClientReleaseStatusConflict},
		{"invalid filename", ClientReleaseDraft, "../bad", r.ID, ErrClientReleaseInvalidFileName},
		{"not declared", ClientReleaseDraft, "qxapi-linux-arm64", r.ID, ErrClientReleaseFileNotInManifest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo.releases[r.ID].Status = tc.status
			_, err := s.CheckUpload(ctx, tc.id, tc.file)
			require.ErrorIs(t, err, tc.want)
			if tc.want != nil {
				_, err = s.Upload(ctx, tc.id, tc.file, bytes.NewReader(nil))
				require.ErrorIs(t, err, tc.want)
			}
		})
	}
}
func TestClientReleaseLifecycleAndRetention(t *testing.T) {
	s, repo := newClientReleaseTestService(t)
	ctx := context.Background()
	data := []byte("release content")
	_, err := s.Latest(ctx)
	require.ErrorIs(t, err, ErrClientReleaseNotFound)
	r, err := s.Create(ctx, clientTestManifest("0.1.0", data))
	require.NoError(t, err)
	_, err = s.Create(ctx, clientTestManifest("v0.1.0", data))
	require.ErrorIs(t, err, ErrClientReleaseVersionExists)
	_, _, err = s.Publish(ctx, r.ID)
	require.ErrorIs(t, err, ErrClientReleaseFilesMissing)
	_, err = s.Withdraw(ctx, r.ID)
	require.ErrorIs(t, err, ErrClientReleaseStatusConflict)
	_, err = s.Upload(ctx, r.ID, clientTestFile, bytes.NewReader(data))
	require.NoError(t, err)
	_, err = s.UpdateNotes(ctx, r.ID, "中文 **说明**")
	require.NoError(t, err)
	published, removed, err := s.Publish(ctx, r.ID)
	require.NoError(t, err)
	require.Empty(t, removed)
	require.NotNil(t, published.PublishedAt)
	_, _, err = s.Publish(ctx, r.ID)
	require.ErrorIs(t, err, ErrClientReleaseStatusConflict)
	require.ErrorIs(t, s.Delete(ctx, r.ID), ErrClientReleaseStatusConflict)
	latest, err := s.Latest(ctx)
	require.NoError(t, err)
	require.Equal(t, "https://panel.example/downloads", latest.URL)
	require.Equal(t, "中文 **说明**", latest.Notes)
	require.Equal(t, "/api/v1/client-releases/qxapi/files/0.1.0/"+clientTestFile, latest.Assets[clientTestFile].URL)
	_, err = s.Withdraw(ctx, r.ID)
	require.NoError(t, err)
	_, err = s.Latest(ctx)
	require.ErrorIs(t, err, ErrClientReleaseNotFound)
	_, err = s.FilePath(ctx, r.Version, clientTestFile)
	require.ErrorIs(t, err, ErrClientReleaseNotFound)
	_, err = s.Upload(ctx, r.ID, clientTestFile, bytes.NewReader(data))
	require.NoError(t, err)
	// Drafts do not count. The withdrawn 0.1.0 DOES count and is removed on the sixth publication.
	draft, err := s.Create(ctx, clientTestManifest("9.0.0", data))
	require.NoError(t, err)
	for i := 1; i <= 5; i++ {
		release, err := s.Create(ctx, clientTestManifest(fmt.Sprintf("0.1.%d", i), data))
		require.NoError(t, err)
		_, err = s.Upload(ctx, release.ID, clientTestFile, bytes.NewReader(data))
		require.NoError(t, err)
		_, removed, err = s.Publish(ctx, release.ID)
		require.NoError(t, err)
		if i < 5 {
			require.Empty(t, removed)
		} else {
			require.Equal(t, []string{"0.1.0"}, removed)
		}
	}
	require.NotContains(t, repo.releases, r.ID)
	require.NoDirExists(t, s.releaseDir(r))
	require.Contains(t, repo.releases, draft.ID)
	require.Len(t, repo.releases, 6)
	latest, err = s.Latest(ctx)
	require.NoError(t, err)
	require.Equal(t, "0.1.5", latest.Version)
	releases, err := s.List(ctx)
	require.NoError(t, err)
	require.Equal(t, "9.0.0", releases[0].Version)
	require.NoError(t, s.Delete(ctx, draft.ID))
	require.NotContains(t, repo.releases, draft.ID)
	last, _ := repo.GetByVersion(ctx, "0.1.5")
	_, err = s.Withdraw(ctx, last.ID)
	require.NoError(t, err)
	latest, err = s.Latest(ctx)
	require.NoError(t, err)
	require.Equal(t, "0.1.4", latest.Version)
	_, _, err = s.Publish(ctx, last.ID)
	require.NoError(t, err)
	require.NoError(t, s.repo.Withdraw(ctx, last.ID))
	require.NoError(t, s.Delete(ctx, last.ID))
	require.NoDirExists(t, s.releaseDir(last))
}
func TestClientReleaseNotes(t *testing.T) {
	s, repo := newClientReleaseTestService(t)
	ctx := context.Background()
	notes, err := s.Notes(ctx, "", "", 0)
	require.NoError(t, err)
	require.NotNil(t, notes)
	require.Empty(t, notes)
	for _, v := range []string{"1.0.0-beta.9", "1.0.0-beta.10", "1.0.0", "1.1.0", "2.0.0", "3.0.0"} {
		r, err := s.Create(ctx, clientTestManifest(v, []byte("x")))
		require.NoError(t, err)
		repo.releases[r.ID].Status = ClientReleasePublished
	}
	repo.releases[5].Status = ClientReleaseWithdrawn
	repo.releases[6].Status = ClientReleaseDraft
	for _, tc := range []struct {
		name, after, upto string
		limit             int
		want              []string
	}{
		{"all", "", "", 0, []string{"1.1.0", "1.0.0", "1.0.0-beta.10", "1.0.0-beta.9"}},
		{"after exclusive", "1.0.0-beta.9", "", 0, []string{"1.1.0", "1.0.0", "1.0.0-beta.10"}},
		{"upto inclusive", "", "1.0.0-beta.10", 0, []string{"1.0.0-beta.10", "1.0.0-beta.9"}},
		{"combined", "1.0.0-beta.9", "v1.0.0", 1, []string{"1.0.0"}},
		{"none", "3.0.0", "", 0, []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			notes, err := s.Notes(ctx, tc.after, tc.upto, tc.limit)
			require.NoError(t, err)
			versions := []string{}
			for _, n := range notes {
				versions = append(versions, n.Version)
				require.Equal(t, "https://panel.example/downloads", n.URL)
			}
			require.Equal(t, tc.want, versions)
		})
	}
	_, err = s.Notes(ctx, "dev", "", 0)
	require.ErrorIs(t, err, ErrClientReleaseInvalidVersion)
	_, err = s.Notes(ctx, "", "../x", 0)
	require.ErrorIs(t, err, ErrClientReleaseInvalidVersion)
}
func TestClientReleasePublicationFailure(t *testing.T) {
	s, repo := newClientReleaseTestService(t)
	ctx := context.Background()
	r, err := s.Create(ctx, clientTestManifest("1.0.0", []byte("hello")))
	require.NoError(t, err)
	repo.saveErr = fmt.Errorf("database unavailable")
	_, err = s.Upload(ctx, r.ID, clientTestFile, strings.NewReader("hello"))
	require.ErrorIs(t, err, repo.saveErr)
	require.NoFileExists(t, filepath.Join(s.releaseDir(r), clientTestFile)+".part")
	_, _, err = s.Publish(ctx, r.ID)
	require.ErrorIs(t, err, ErrClientReleaseFilesMissing)
	repo.saveErr = nil
	_, err = s.Upload(ctx, r.ID, clientTestFile, strings.NewReader("hello"))
	require.NoError(t, err)
	repo.publishErr = fmt.Errorf("transaction failed")
	_, _, err = s.Publish(ctx, r.ID)
	require.ErrorIs(t, err, repo.publishErr)
	require.Equal(t, ClientReleaseDraft, repo.releases[r.ID].Status)
	require.NoError(t, os.Remove(filepath.Join(s.releaseDir(r), clientTestFile)))
	_, _, err = s.Publish(ctx, r.ID)
	require.ErrorIs(t, err, ErrClientReleaseFilesMissing)
}

func TestClientReleaseConcurrentUploadsAndPublish(t *testing.T) {
	s, repo := newClientReleaseTestService(t)
	ctx := context.Background()
	release, err := s.Create(ctx, clientTestManifest("1.0.0", []byte("hello")))
	require.NoError(t, err)
	reader, writer := io.Pipe()
	uploaded := make(chan error, 1)
	go func() {
		_, err := s.Upload(ctx, release.ID, clientTestFile, reader)
		_ = reader.Close()
		uploaded <- err
	}()
	// The first write completes only once Upload is streaming under its mutation lock.
	_, err = writer.Write([]byte("he"))
	require.NoError(t, err)
	overwritten := make(chan error, 1)
	go func() {
		_, err := s.Upload(ctx, release.ID, clientTestFile, strings.NewReader("hello"))
		overwritten <- err
	}()
	_, err = writer.Write([]byte("llo"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	require.NoError(t, <-uploaded)
	require.NoError(t, <-overwritten)
	_, _, err = s.Publish(ctx, release.ID)
	require.NoError(t, err)
	require.Len(t, repo.releases[release.ID].Files, 1)
	data, err := os.ReadFile(filepath.Join(s.releaseDir(release), clientTestFile))
	require.NoError(t, err)
	require.Equal(t, "hello", string(data))
	require.NoFileExists(t, filepath.Join(s.releaseDir(release), clientTestFile)+".part")
}

func TestClientReleasePublishTooOldPreservesStateAndFiles(t *testing.T) {
	for _, withdrawn := range []int{0, 1, 5} {
		t.Run(fmt.Sprintf("withdrawn=%d", withdrawn), func(t *testing.T) {
			s, repo := newClientReleaseTestService(t)
			ctx := context.Background()
			content := []byte("retained release file")
			for i := 2; i <= 6; i++ {
				r, err := s.Create(ctx, clientTestManifest(fmt.Sprintf("0.9.%d", i), content))
				require.NoError(t, err)
				_, err = s.Upload(ctx, r.ID, clientTestFile, bytes.NewReader(content))
				require.NoError(t, err)
				_, _, err = s.Publish(ctx, r.ID)
				require.NoError(t, err)
				if i-2 < withdrawn {
					_, err = s.Withdraw(ctx, r.ID)
					require.NoError(t, err)
				}
			}
			old, err := s.Create(ctx, clientTestManifest("0.0.1", content))
			require.NoError(t, err)
			_, err = s.Upload(ctx, old.ID, clientTestFile, bytes.NewReader(content))
			require.NoError(t, err)
			before, err := s.List(ctx)
			require.NoError(t, err)
			// A repository publication would mutate state; reject before invoking it.
			repo.publishErr = fmt.Errorf("publication must not be called for an old version")
			release, removed, err := s.Publish(ctx, old.ID)
			require.ErrorIs(t, err, ErrClientReleaseTooOld)
			require.EqualValues(t, http.StatusConflict, ErrClientReleaseTooOld.Code)
			require.Equal(t, "Release is older than the five retained versions", ErrClientReleaseTooOld.Message)
			require.Nil(t, release)
			require.Nil(t, removed)
			after, err := s.List(ctx)
			require.NoError(t, err)
			require.Equal(t, before, after)
			require.Equal(t, ClientReleaseDraft, repo.releases[old.ID].Status)
			require.Nil(t, repo.releases[old.ID].PublishedAt)
			for _, r := range before {
				path := filepath.Join(s.releaseDir(r), clientTestFile)
				data, err := os.ReadFile(path)
				require.NoError(t, err)
				require.Equal(t, content, data)
				require.NoFileExists(t, path+".part")
			}
		})
	}
}

func TestClientReleasePublishOldVersionWithinFive(t *testing.T) {
	s, _ := newClientReleaseTestService(t)
	ctx := context.Background()
	content := []byte("release content")
	for i := 2; i <= 5; i++ {
		r, err := s.Create(ctx, clientTestManifest(fmt.Sprintf("0.9.%d", i), content))
		require.NoError(t, err)
		_, err = s.Upload(ctx, r.ID, clientTestFile, bytes.NewReader(content))
		require.NoError(t, err)
		_, _, err = s.Publish(ctx, r.ID)
		require.NoError(t, err)
	}
	// An unrelated newer draft must not consume a retention slot.
	_, err := s.Create(ctx, clientTestManifest("9.0.0", content))
	require.NoError(t, err)
	old, err := s.Create(ctx, clientTestManifest("0.0.1", content))
	require.NoError(t, err)
	_, err = s.Upload(ctx, old.ID, clientTestFile, bytes.NewReader(content))
	require.NoError(t, err)
	release, removed, err := s.Publish(ctx, old.ID)
	require.NoError(t, err)
	require.Equal(t, ClientReleasePublished, release.Status)
	require.Empty(t, removed)
	require.FileExists(t, filepath.Join(s.releaseDir(old), clientTestFile))
}

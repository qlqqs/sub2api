package service

// CUSTOM: QxAPI release lifecycle and filesystem storage.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrClientReleaseInvalidManifest   = infraerrors.BadRequest("CLIENT_RELEASE_INVALID_MANIFEST", "Invalid release manifest")
	ErrClientReleaseInvalidVersion    = infraerrors.BadRequest("CLIENT_RELEASE_INVALID_VERSION", "Invalid release version")
	ErrClientReleaseInvalidFileName   = infraerrors.BadRequest("CLIENT_RELEASE_INVALID_FILE_NAME", "Invalid release file name")
	ErrClientReleaseVersionExists     = infraerrors.Conflict("CLIENT_RELEASE_VERSION_EXISTS", "Release version already exists")
	ErrClientReleaseNotFound          = infraerrors.NotFound("CLIENT_RELEASE_NOT_FOUND", "Release not found")
	ErrClientReleaseFileNotInManifest = infraerrors.BadRequest("CLIENT_RELEASE_FILE_NOT_IN_MANIFEST", "File is not in the release manifest")
	ErrClientReleaseFileMismatch      = infraerrors.BadRequest("CLIENT_RELEASE_FILE_MISMATCH", "File size or SHA256 does not match the manifest")
	ErrClientReleaseFilesMissing      = infraerrors.BadRequest("CLIENT_RELEASE_FILES_MISSING", "Release files are missing")
	ErrClientReleaseStatusConflict    = infraerrors.Conflict("CLIENT_RELEASE_STATUS_CONFLICT", "Release status does not allow this operation")
	ErrClientReleaseTooOld            = infraerrors.Conflict("CLIENT_RELEASE_TOO_OLD", "Release is older than the five retained versions")
	clientReleaseFileName             = regexp.MustCompile(`^qxapi-(windows|darwin|macos|linux)-(amd64|arm64)(\.exe|\.zip|\.dmg)?$`)
	clientReleaseVersionPath          = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]*)?$`)
	clientReleaseSHA256               = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

const (
	ClientReleaseDraft     = "draft"
	ClientReleasePublished = "published"
	ClientReleaseWithdrawn = "withdrawn"
)

type ClientReleaseFileSpec struct {
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type ClientReleaseManifest struct {
	Version string                           `json:"version"`
	Files   map[string]ClientReleaseFileSpec `json:"files"`
}
type ClientReleaseFile struct {
	Name string
	ClientReleaseFileSpec
}
type ClientRelease struct {
	ID          int64
	Product     string
	Version     string
	Notes       string
	Manifest    json.RawMessage
	Status      string
	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Files       []ClientReleaseFile
}

type ClientReleaseRepository interface {
	List(context.Context) ([]*ClientRelease, error)
	GetByID(context.Context, int64) (*ClientRelease, error)
	GetByVersion(context.Context, string) (*ClientRelease, error)
	Create(context.Context, string, json.RawMessage) (*ClientRelease, error)
	SaveFile(context.Context, int64, ClientReleaseFile) error
	UpdateNotes(context.Context, int64, string) error
	Publish(context.Context, int64, []int64) error
	Withdraw(context.Context, int64) error
	Delete(context.Context, int64) error
}

type ClientReleaseService struct {
	repo     ClientReleaseRepository
	settings *SettingService
	root     string
	// The deployment has one application instance. Serialize mutations, including
	// streaming uploads, so publish/delete cannot race an in-flight .part file.
	mu sync.Mutex
}

func NewClientReleaseService(repo ClientReleaseRepository, settings *SettingService) *ClientReleaseService {
	base := strings.TrimSpace(os.Getenv("DATA_DIR"))
	if base == "" {
		base = "./data"
	}
	return &ClientReleaseService{repo: repo, settings: settings, root: filepath.Join(base, "client-releases")}
}

func normalizeClientReleaseVersion(v string) (string, error) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if len(v) > 64 || !clientReleaseVersionPath.MatchString(v) || strings.Contains(v, "+") || strings.Contains(v, "dirty") || !clientVersionReleased(v) {
		return "", ErrClientReleaseInvalidVersion
	}
	return v, nil
}
func ParseClientReleaseManifest(raw []byte) (*ClientReleaseManifest, error) {
	var m ClientReleaseManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, ErrClientReleaseInvalidManifest
	}
	version, err := normalizeClientReleaseVersion(m.Version)
	if err != nil {
		return nil, err
	}
	m.Version = version
	if len(m.Files) == 0 {
		return nil, ErrClientReleaseInvalidManifest
	}
	for name, f := range m.Files {
		if !clientReleaseFileName.MatchString(name) {
			return nil, ErrClientReleaseInvalidFileName
		}
		if f.Size <= 0 || !clientReleaseSHA256.MatchString(f.SHA256) {
			return nil, ErrClientReleaseInvalidManifest
		}
	}
	return &m, nil
}
func (s *ClientReleaseService) List(ctx context.Context) ([]*ClientRelease, error) {
	releases, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(releases, func(i, j int) bool { return clientVersionNewer(releases[i].Version, releases[j].Version) })
	return releases, nil
}
func (s *ClientReleaseService) Create(ctx context.Context, raw json.RawMessage) (*ClientRelease, error) {
	m, err := ParseClientReleaseManifest(raw)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.repo.Create(ctx, m.Version, raw)
}
func (s *ClientReleaseService) checkUpload(ctx context.Context, id int64, name string) (*ClientRelease, ClientReleaseFile, error) {
	r, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, ClientReleaseFile{}, err
	}
	if r.Status != ClientReleaseDraft && r.Status != ClientReleaseWithdrawn {
		return nil, ClientReleaseFile{}, ErrClientReleaseStatusConflict
	}
	if !clientReleaseFileName.MatchString(name) {
		return nil, ClientReleaseFile{}, ErrClientReleaseInvalidFileName
	}
	m, err := ParseClientReleaseManifest(r.Manifest)
	if err != nil {
		return nil, ClientReleaseFile{}, err
	}
	for storedName, spec := range m.Files {
		if storedName == name {
			return r, ClientReleaseFile{Name: storedName, ClientReleaseFileSpec: spec}, nil
		}
	}
	return nil, ClientReleaseFile{}, ErrClientReleaseFileNotInManifest
}
func (s *ClientReleaseService) CheckUpload(ctx context.Context, id int64, name string) (ClientReleaseFileSpec, error) {
	_, f, err := s.checkUpload(ctx, id, name)
	return f.ClientReleaseFileSpec, err
}
func (s *ClientReleaseService) releaseDir(r *ClientRelease) string {
	return filepath.Join(s.root, r.Product, r.Version)
}
func (s *ClientReleaseService) Upload(ctx context.Context, id int64, name string, body io.Reader) (*ClientRelease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, file, err := s.checkUpload(ctx, id, name)
	if err != nil {
		return nil, err
	}
	expected := file.ClientReleaseFileSpec
	dir := s.releaseDir(r)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("create release directory: %w", err)
	}
	path := filepath.Join(dir, file.Name)
	part := path + ".part"
	f, err := os.OpenFile(part, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("create release upload: %w", err)
	}
	defer func() { _ = os.Remove(part) }()
	hash := sha256.New()
	// The HTTP boundary also uses MaxBytesReader. Bound service callers too,
	// and read the extra byte separately to avoid size+1 integer overflow.
	size, copyErr := io.Copy(io.MultiWriter(f, hash), io.LimitReader(body, expected.Size))
	if copyErr == nil {
		var extra [1]byte
		n, readErr := io.ReadFull(body, extra[:])
		if n > 0 {
			copyErr = ErrClientReleaseFileMismatch
		} else if readErr != io.EOF {
			copyErr = readErr
		}
	}
	closeErr := f.Close()
	var tooLarge *http.MaxBytesError
	if errors.As(copyErr, &tooLarge) || errors.Is(copyErr, ErrClientReleaseFileMismatch) || errors.Is(copyErr, io.ErrUnexpectedEOF) {
		return nil, ErrClientReleaseFileMismatch
	}
	if copyErr != nil {
		return nil, fmt.Errorf("read release upload: %w", copyErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close release upload: %w", closeErr)
	}
	if size != expected.Size || hex.EncodeToString(hash.Sum(nil)) != expected.SHA256 {
		return nil, ErrClientReleaseFileMismatch
	}
	if err := os.Rename(part, path); err != nil {
		return nil, fmt.Errorf("finish release upload: %w", err)
	}
	if err := s.repo.SaveFile(ctx, id, file); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}
func (s *ClientReleaseService) UpdateNotes(ctx context.Context, id int64, notes string) (*ClientRelease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.repo.UpdateNotes(ctx, id, notes); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}
func (s *ClientReleaseService) Publish(ctx context.Context, id int64) (*ClientRelease, []string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if r.Status != ClientReleaseDraft && r.Status != ClientReleaseWithdrawn {
		return nil, nil, ErrClientReleaseStatusConflict
	}
	m, err := ParseClientReleaseManifest(r.Manifest)
	if err != nil {
		return nil, nil, err
	}
	uploaded := make(map[string]ClientReleaseFile, len(r.Files))
	for _, f := range r.Files {
		uploaded[f.Name] = f
	}
	for name, expected := range m.Files {
		f, ok := uploaded[name]
		if !ok || f.ClientReleaseFileSpec != expected {
			return nil, nil, ErrClientReleaseFilesMissing
		}
		info, err := os.Stat(filepath.Join(s.releaseDir(r), f.Name))
		if os.IsNotExist(err) {
			return nil, nil, ErrClientReleaseFilesMissing
		}
		if err != nil {
			return nil, nil, err
		}
		if !info.Mode().IsRegular() || info.Size() != expected.Size {
			return nil, nil, ErrClientReleaseFilesMissing
		}
	}
	releases, err := s.List(ctx)
	if err != nil {
		return nil, nil, err
	}
	removed := make([]*ClientRelease, 0)
	ids := make([]int64, 0)
	versions := make([]string, 0)
	count := 0
	for _, candidate := range releases {
		if candidate.Status == ClientReleaseDraft && candidate.ID != id {
			continue
		}
		count++
		if count > 5 {
			if candidate.ID == id {
				return nil, nil, ErrClientReleaseTooOld
			}
			removed = append(removed, candidate)
			ids = append(ids, candidate.ID)
			versions = append(versions, candidate.Version)
		}
	}
	if err := s.repo.Publish(ctx, id, ids); err != nil {
		return nil, nil, err
	}
	for _, old := range removed {
		if err := os.RemoveAll(s.releaseDir(old)); err != nil {
			return nil, nil, fmt.Errorf("remove release files: %w", err)
		}
	}
	r, err = s.repo.GetByID(ctx, id)
	return r, versions, err
}
func (s *ClientReleaseService) Withdraw(ctx context.Context, id int64) (*ClientRelease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if r.Status != ClientReleasePublished {
		return nil, ErrClientReleaseStatusConflict
	}
	if err := s.repo.Withdraw(ctx, id); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}
func (s *ClientReleaseService) Delete(ctx context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if r.Status != ClientReleaseDraft && r.Status != ClientReleaseWithdrawn {
		return ErrClientReleaseStatusConflict
	}
	if err := os.RemoveAll(s.releaseDir(r)); err != nil {
		return fmt.Errorf("remove release files: %w", err)
	}
	return s.repo.Delete(ctx, id)
}

type ClientReleaseNote struct {
	Version string `json:"version"`
	Notes   string `json:"notes"`
	URL     string `json:"url"`
}
type ClientReleaseAsset struct {
	URL string `json:"url"`
	ClientReleaseFileSpec
}
type ClientReleaseLatest struct {
	ClientReleaseNote
	Assets map[string]ClientReleaseAsset `json:"assets"`
}

func (s *ClientReleaseService) downloadsURL(ctx context.Context) string {
	return strings.TrimRight(s.settings.GetFrontendURL(ctx), "/") + "/downloads"
}
func (s *ClientReleaseService) Latest(ctx context.Context) (*ClientReleaseLatest, error) {
	releases, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range releases {
		if r.Status != ClientReleasePublished {
			continue
		}
		result := &ClientReleaseLatest{ClientReleaseNote: ClientReleaseNote{r.Version, r.Notes, s.downloadsURL(ctx)}, Assets: make(map[string]ClientReleaseAsset)}
		for _, f := range r.Files {
			result.Assets[f.Name] = ClientReleaseAsset{URL: "/api/v1/client-releases/qxapi/files/" + r.Version + "/" + f.Name, ClientReleaseFileSpec: f.ClientReleaseFileSpec}
		}
		return result, nil
	}
	return nil, ErrClientReleaseNotFound
}
func (s *ClientReleaseService) Notes(ctx context.Context, after, upto string, limit int) ([]ClientReleaseNote, error) {
	for _, v := range []string{after, upto} {
		if v != "" {
			if _, err := normalizeClientReleaseVersion(v); err != nil {
				return nil, err
			}
		}
	}
	releases, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	notes := make([]ClientReleaseNote, 0)
	url := s.downloadsURL(ctx)
	for _, r := range releases {
		if r.Status != ClientReleasePublished || (after != "" && !clientVersionNewer(r.Version, after)) || (upto != "" && clientVersionNewer(r.Version, upto)) {
			continue
		}
		notes = append(notes, ClientReleaseNote{r.Version, r.Notes, url})
		if limit > 0 && len(notes) >= limit {
			break
		}
	}
	return notes, nil
}
func (s *ClientReleaseService) FilePath(ctx context.Context, version, name string) (string, error) {
	normalized, err := normalizeClientReleaseVersion(version)
	if err != nil {
		return "", err
	}
	if !clientReleaseFileName.MatchString(name) {
		return "", ErrClientReleaseInvalidFileName
	}
	r, err := s.repo.GetByVersion(ctx, normalized)
	if err != nil {
		return "", err
	}
	if r.Status != ClientReleasePublished {
		return "", ErrClientReleaseNotFound
	}
	for _, f := range r.Files {
		if f.Name == name {
			return filepath.Join(s.releaseDir(r), f.Name), nil
		}
	}
	return "", ErrClientReleaseNotFound
}

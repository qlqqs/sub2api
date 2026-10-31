//go:build unit

package repository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

var clientReleaseColumns = []string{"id", "product", "version", "notes", "manifest", "status", "published_at", "created_at", "updated_at"}

const clientRepoManifest = `{"version":"1.0.0","files":{"qxapi-linux-amd64":{"size":1,"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}}`

func clientReleaseRows() *sqlmock.Rows {
	now := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	return sqlmock.NewRows(clientReleaseColumns).AddRow(int64(1), "qxapi", "1.0.0", "notes", []byte(clientRepoManifest), "draft", nil, now, now)
}
func clientReleaseMock(t *testing.T) (*clientReleaseRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, m, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, m.ExpectationsWereMet()); _ = db.Close() })
	return &clientReleaseRepository{db}, m
}
func TestClientReleaseRepositoryCreate(t *testing.T) {
	for _, tc := range []struct {
		name        string
		dbErr, want error
	}{{"success", nil, nil}, {"duplicate", &pq.Error{Code: "23505"}, service.ErrClientReleaseVersionExists}, {"database", errors.New("offline"), errors.New("offline")}} {
		t.Run(tc.name, func(t *testing.T) {
			r, m := clientReleaseMock(t)
			q := m.ExpectQuery("INSERT INTO client_releases").WithArgs("1.0.0", []byte(clientRepoManifest))
			if tc.dbErr != nil {
				q.WillReturnError(tc.dbErr)
			} else {
				q.WillReturnRows(clientReleaseRows())
			}
			got, err := r.Create(context.Background(), "1.0.0", []byte(clientRepoManifest))
			if tc.name == "database" {
				require.ErrorIs(t, err, tc.dbErr)
			} else {
				require.ErrorIs(t, err, tc.want)
			}
			if err == nil {
				require.Equal(t, "1.0.0", got.Version)
				require.Nil(t, got.PublishedAt)
				require.JSONEq(t, clientRepoManifest, string(got.Manifest))
				require.Empty(t, got.Files)
			}
		})
	}
}
func TestClientReleaseRepositoryReads(t *testing.T) {
	ctx := context.Background()
	t.Run("list with files", func(t *testing.T) {
		r, m := clientReleaseMock(t)
		m.ExpectQuery(regexp.QuoteMeta(clientReleaseSelect) + " WHERE product").WillReturnRows(clientReleaseRows())
		m.ExpectQuery("SELECT f.release_id").WillReturnRows(sqlmock.NewRows([]string{"release_id", "name", "size", "sha256"}).AddRow(1, "qxapi-linux-amd64", 1, "hash"))
		got, err := r.List(ctx)
		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, "qxapi-linux-amd64", got[0].Files[0].Name)
	})
	t.Run("empty array", func(t *testing.T) {
		r, m := clientReleaseMock(t)
		m.ExpectQuery("SELECT id").WillReturnRows(sqlmock.NewRows(clientReleaseColumns))
		got, err := r.List(ctx)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Empty(t, got)
	})
	for _, byVersion := range []bool{false, true} {
		t.Run("get", func(t *testing.T) {
			r, m := clientReleaseMock(t)
			q := m.ExpectQuery("SELECT id")
			if byVersion {
				q.WithArgs("1.0.0")
			} else {
				q.WithArgs(int64(1))
			}
			q.WillReturnRows(clientReleaseRows())
			m.ExpectQuery("SELECT name, size, sha256").WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"name", "size", "sha256"}).AddRow("qxapi-linux-amd64", 1, "hash"))
			var got *service.ClientRelease
			var err error
			if byVersion {
				got, err = r.GetByVersion(ctx, "1.0.0")
			} else {
				got, err = r.GetByID(ctx, 1)
			}
			require.NoError(t, err)
			require.Len(t, got.Files, 1)
		})
	}
	t.Run("missing", func(t *testing.T) {
		r, m := clientReleaseMock(t)
		m.ExpectQuery("SELECT id").WithArgs(int64(99)).WillReturnError(sql.ErrNoRows)
		_, err := r.GetByID(ctx, 99)
		require.ErrorIs(t, err, service.ErrClientReleaseNotFound)
	})
	t.Run("row error", func(t *testing.T) {
		r, m := clientReleaseMock(t)
		boom := errors.New("read failed")
		m.ExpectQuery("SELECT id").WillReturnRows(clientReleaseRows().RowError(0, boom))
		_, err := r.List(ctx)
		require.ErrorIs(t, err, boom)
	})
	t.Run("file row error", func(t *testing.T) {
		r, m := clientReleaseMock(t)
		boom := errors.New("file read failed")
		m.ExpectQuery("SELECT id").WillReturnRows(clientReleaseRows())
		m.ExpectQuery("SELECT f.release_id").WillReturnRows(sqlmock.NewRows([]string{"release_id", "name", "size", "sha256"}).AddRow(1, "qxapi-linux-amd64", 1, "hash").RowError(0, boom))
		_, err := r.List(ctx)
		require.ErrorIs(t, err, boom)
	})
}
func TestClientReleaseRepositorySaveFile(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{true: "rollback", false: "commit"}[fail], func(t *testing.T) {
			r, m := clientReleaseMock(t)
			m.ExpectBegin()
			m.ExpectExec("UPDATE client_releases SET updated_at").WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
			q := m.ExpectExec("INSERT INTO client_release_files .* ON CONFLICT").WithArgs(int64(1), "qxapi-linux-amd64", int64(5), "hash")
			boom := errors.New("write failed")
			if fail {
				q.WillReturnError(boom)
				m.ExpectRollback()
			} else {
				q.WillReturnResult(sqlmock.NewResult(1, 1))
				m.ExpectCommit()
			}
			err := r.SaveFile(context.Background(), 1, service.ClientReleaseFile{Name: "qxapi-linux-amd64", ClientReleaseFileSpec: service.ClientReleaseFileSpec{Size: 5, SHA256: "hash"}})
			if fail {
				require.ErrorIs(t, err, boom)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
func TestClientReleaseRepositoryPublishAtomicCleanup(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{true: "rollback", false: "commit"}[fail], func(t *testing.T) {
			r, m := clientReleaseMock(t)
			m.ExpectBegin()
			m.ExpectExec("UPDATE client_releases SET status = 'published'").WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec("DELETE FROM client_releases").WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
			q := m.ExpectExec("DELETE FROM client_releases").WithArgs(int64(2))
			boom := errors.New("delete failed")
			if fail {
				q.WillReturnError(boom)
				m.ExpectRollback()
			} else {
				q.WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectCommit()
			}
			err := r.Publish(context.Background(), 7, []int64{1, 2})
			if fail {
				require.ErrorIs(t, err, boom)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
func TestClientReleaseRepositoryMutations(t *testing.T) {
	for _, op := range []string{"notes", "withdraw", "delete"} {
		for _, affected := range []int64{0, 1} {
			t.Run(op, func(t *testing.T) {
				r, m := clientReleaseMock(t)
				ctx := context.Background()
				var err error
				switch op {
				case "notes":
					m.ExpectExec("UPDATE client_releases SET notes").WithArgs(int64(1), "中文").WillReturnResult(sqlmock.NewResult(0, affected))
					err = r.UpdateNotes(ctx, 1, "中文")
				case "withdraw":
					m.ExpectExec("UPDATE client_releases SET status = 'withdrawn'").WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, affected))
					err = r.Withdraw(ctx, 1)
				case "delete":
					m.ExpectExec("DELETE FROM client_releases").WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, affected))
					err = r.Delete(ctx, 1)
				}
				if affected == 0 {
					require.ErrorIs(t, err, service.ErrClientReleaseNotFound)
				} else {
					require.NoError(t, err)
				}
			})
		}
	}
}

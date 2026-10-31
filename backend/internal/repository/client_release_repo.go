package repository

// CUSTOM: Fork-owned release tables use native SQL, not Ent schema changes.
import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type clientReleaseRepository struct{ db *sql.DB }

func NewClientReleaseRepository(db *sql.DB) service.ClientReleaseRepository {
	return &clientReleaseRepository{db: db}
}

const clientReleaseSelect = `SELECT id, product, version, notes, manifest, status, published_at, created_at, updated_at FROM client_releases`

type clientReleaseScanner interface{ Scan(...any) error }

func scanClientRelease(row clientReleaseScanner) (*service.ClientRelease, error) {
	r := &service.ClientRelease{Files: make([]service.ClientReleaseFile, 0)}
	err := row.Scan(&r.ID, &r.Product, &r.Version, &r.Notes, &r.Manifest, &r.Status, &r.PublishedAt, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrClientReleaseNotFound, service.ErrClientReleaseVersionExists)
	}
	return r, nil
}
func (r *clientReleaseRepository) List(ctx context.Context) ([]*service.ClientRelease, error) {
	rows, err := r.db.QueryContext(ctx, clientReleaseSelect+` WHERE product = 'qxapi' ORDER BY id`)
	if err != nil {
		return nil, err
	}
	releases := make([]*service.ClientRelease, 0)
	byID := make(map[int64]*service.ClientRelease)
	for rows.Next() {
		release, err := scanClientRelease(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		releases = append(releases, release)
		byID[release.ID] = release
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	if len(releases) == 0 {
		return releases, nil
	}
	files, err := r.db.QueryContext(ctx, `SELECT f.release_id, f.name, f.size, f.sha256 FROM client_release_files f JOIN client_releases r ON r.id = f.release_id WHERE r.product = 'qxapi' ORDER BY f.name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = files.Close() }()
	for files.Next() {
		var id int64
		var f service.ClientReleaseFile
		if err := files.Scan(&id, &f.Name, &f.Size, &f.SHA256); err != nil {
			return nil, err
		}
		if release := byID[id]; release != nil {
			release.Files = append(release.Files, f)
		}
	}
	return releases, files.Err()
}
func (r *clientReleaseRepository) get(ctx context.Context, predicate string, arg any) (*service.ClientRelease, error) {
	release, err := scanClientRelease(r.db.QueryRowContext(ctx, clientReleaseSelect+` WHERE product = 'qxapi' AND `+predicate, arg))
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT name, size, sha256 FROM client_release_files WHERE release_id = $1 ORDER BY name`, release.ID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var f service.ClientReleaseFile
		if err := rows.Scan(&f.Name, &f.Size, &f.SHA256); err != nil {
			return nil, err
		}
		release.Files = append(release.Files, f)
	}
	return release, rows.Err()
}
func (r *clientReleaseRepository) GetByID(ctx context.Context, id int64) (*service.ClientRelease, error) {
	return r.get(ctx, "id = $1", id)
}
func (r *clientReleaseRepository) GetByVersion(ctx context.Context, version string) (*service.ClientRelease, error) {
	return r.get(ctx, "version = $1", version)
}
func (r *clientReleaseRepository) Create(ctx context.Context, version string, manifest json.RawMessage) (*service.ClientRelease, error) {
	return scanClientRelease(r.db.QueryRowContext(ctx, `INSERT INTO client_releases (product, version, manifest, status) VALUES ('qxapi', $1, $2::jsonb, 'draft') RETURNING id, product, version, notes, manifest, status, published_at, created_at, updated_at`, version, []byte(manifest)))
}
func clientReleaseAffected(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return service.ErrClientReleaseNotFound
	}
	return nil
}
func (r *clientReleaseRepository) SaveFile(ctx context.Context, id int64, f service.ClientReleaseFile) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := clientReleaseAffected(tx.ExecContext(ctx, `UPDATE client_releases SET updated_at = NOW() WHERE id = $1 AND product = 'qxapi'`, id)); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO client_release_files (release_id, name, size, sha256) VALUES ($1, $2, $3, $4) ON CONFLICT (release_id, name) DO UPDATE SET size = EXCLUDED.size, sha256 = EXCLUDED.sha256, created_at = NOW()`, id, f.Name, f.Size, f.SHA256)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (r *clientReleaseRepository) UpdateNotes(ctx context.Context, id int64, notes string) error {
	return clientReleaseAffected(r.db.ExecContext(ctx, `UPDATE client_releases SET notes = $2, updated_at = NOW() WHERE id = $1 AND product = 'qxapi'`, id, notes))
}
func (r *clientReleaseRepository) Publish(ctx context.Context, id int64, removed []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := clientReleaseAffected(tx.ExecContext(ctx, `UPDATE client_releases SET status = 'published', published_at = NOW(), updated_at = NOW() WHERE id = $1 AND product = 'qxapi'`, id)); err != nil {
		return err
	}
	for _, oldID := range removed {
		if err := clientReleaseAffected(tx.ExecContext(ctx, `DELETE FROM client_releases WHERE id = $1 AND product = 'qxapi'`, oldID)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (r *clientReleaseRepository) Withdraw(ctx context.Context, id int64) error {
	return clientReleaseAffected(r.db.ExecContext(ctx, `UPDATE client_releases SET status = 'withdrawn', updated_at = NOW() WHERE id = $1 AND product = 'qxapi'`, id))
}
func (r *clientReleaseRepository) Delete(ctx context.Context, id int64) error {
	return clientReleaseAffected(r.db.ExecContext(ctx, `DELETE FROM client_releases WHERE id = $1 AND product = 'qxapi'`, id))
}

var _ service.ClientReleaseRepository = (*clientReleaseRepository)(nil)

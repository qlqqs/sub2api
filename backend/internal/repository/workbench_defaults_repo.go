package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type workbenchRepository struct{ db *sql.DB }

func NewWorkbenchRepository(db *sql.DB) service.WorkbenchRepository { return &workbenchRepository{db} }
func (r *workbenchRepository) Target(ctx context.Context) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `SELECT value::bigint FROM settings WHERE key='signup_workbench_group_id'`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}
func (r *workbenchRepository) ClaimSignup(ctx context.Context, userID, groupID int64) (bool, error) {
	result, err := r.db.ExecContext(ctx, `INSERT INTO workbench_signups(user_id,group_id) VALUES ($1,NULLIF($2,0)) ON CONFLICT DO NOTHING`, userID, groupID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}
func (r *workbenchRepository) State(ctx context.Context, userID int64, kind string) (*service.WorkbenchState, error) {
	state := &service.WorkbenchState{Preferences: json.RawMessage(`{}`)}
	err := r.db.QueryRowContext(ctx, `SELECT COALESCE(s.user_id IS NOT NULL,false),COALESCE(g.id,0),COALESCE(g.name,''),p.user_id IS NOT NULL,COALESCE(p.preferences,'{}'::jsonb)
 FROM users u LEFT JOIN workbench_signups s ON s.user_id=u.id
 LEFT JOIN groups g ON g.id=s.group_id AND g.deleted_at IS NULL AND g.status='active'
 LEFT JOIN workbench_preferences p ON p.user_id=u.id AND p.workbench=$2
 WHERE u.id=$1 AND u.deleted_at IS NULL`, userID, kind).Scan(&state.Eligible, &state.GroupID, &state.GroupName, &state.Initialized, &state.Preferences)
	return state, err
}
func (r *workbenchRepository) Save(ctx context.Context, userID int64, kind string, prefs json.RawMessage, initialize bool) (json.RawMessage, error) {
	query := `INSERT INTO workbench_preferences(user_id,workbench,preferences) VALUES ($1,$2,$3) ON CONFLICT(user_id,workbench) DO UPDATE SET preferences=EXCLUDED.preferences,updated_at=NOW() RETURNING preferences`
	if initialize {
		// A simultaneous explicit save wins; initializing twice returns the existing preference.
		query = `INSERT INTO workbench_preferences(user_id,workbench,preferences) VALUES ($1,$2,$3) ON CONFLICT(user_id,workbench) DO UPDATE SET preferences=workbench_preferences.preferences RETURNING preferences`
	}
	var out json.RawMessage
	err := r.db.QueryRowContext(ctx, query, userID, kind, string(prefs)).Scan(&out)
	return out, err
}

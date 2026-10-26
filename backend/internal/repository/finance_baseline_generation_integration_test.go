//go:build integration

package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFinanceBaselineDeletionGeneration(t *testing.T) {
	financeCoverageFixture(t)
	f := financeRecalcData(t)
	financeRecalcUsage(t, f)
	ctx := context.Background()
	read := func(q interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
	}) (int64, time.Time) {
		var generation int64
		var frontier time.Time
		require.NoError(t, q.QueryRowContext(ctx, `SELECT deletion_generation,complete_from FROM finance_aggregation_coverage_state WHERE id=1`).Scan(&generation, &frontier))
		return generation, frontier
	}
	original, frontier := read(integrationDB)
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `DELETE FROM usage_logs WHERE account_id=$1 AND model='no-such-generation-model'`, f.accountID)
	require.NoError(t, err)
	generation, _ := read(tx)
	require.Equal(t, original, generation, "zero-row DELETE is not source loss")
	_, err = tx.ExecContext(ctx, `DELETE FROM usage_logs WHERE account_id=$1`, f.accountID)
	require.NoError(t, err)
	generation, afterFrontier := read(tx)
	require.Equal(t, original+1, generation)
	require.Equal(t, frontier, afterFrontier, "old deletion increments generation even when frontier was already later")
	require.NoError(t, tx.Rollback())
	generation, afterFrontier = read(integrationDB)
	require.Equal(t, original, generation, "generation and source deletion roll back together")
	require.Equal(t, frontier, afterFrontier)

	tx, err = integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	// Isolated testcontainers transaction only; all dependent data restored.
	_, err = tx.ExecContext(ctx, `TRUNCATE usage_logs CASCADE`)
	require.NoError(t, err)
	generation, _ = read(tx)
	require.Equal(t, original+1, generation)
	require.NoError(t, tx.Rollback())
	generation, _ = read(integrationDB)
	require.Equal(t, original, generation)
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM usage_logs WHERE account_id=$1`, f.accountID).Scan(&count))
	require.Equal(t, 2, count)
}

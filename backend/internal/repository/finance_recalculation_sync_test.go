//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestFinanceAggregationLockAndUnsafeMembership(t *testing.T) {
	db, mock := newSQLMock(t)
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	mock.ExpectExec(`LOCK TABLE usage_logs IN ROW SHARE MODE`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WithArgs(financeAggregationLockID).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, AcquireFinanceAggregationLock(context.Background(), tx))
	batch := &service.FinanceRecalculationBatch{FinanceRecalculationFilter: service.FinanceRecalculationFilter{Kind: service.FinanceRecalculationUsageCost}}
	require.NoError(t, SynchronizeFinanceCostAggregates(context.Background(), tx, batch, []service.FinanceRecalculationItem{{Status: service.FinanceRecalculationUnchanged}, {Status: service.FinanceRecalculationSkipped}}))
	mock.ExpectQuery(`SELECT complete_from FROM finance_aggregation_coverage_state`).WillReturnRows(sqlmock.NewRows([]string{"complete_from"}).AddRow(time.Now()))
	require.ErrorIs(t, SynchronizeFinanceCostAggregates(context.Background(), tx, batch, []service.FinanceRecalculationItem{{Status: service.FinanceRecalculationChanged, Candidate: service.FinanceRecalculationCandidate{TargetCreatedAt: time.Unix(0, 0)}}}), service.ErrFinanceRecalculationCoverageUnknown)
	batch.Kind = service.FinanceRecalculationPaymentFee
	require.NoError(t, SynchronizeFinanceCostAggregates(context.Background(), tx, batch, []service.FinanceRecalculationItem{{Status: service.FinanceRecalculationChanged}}))
	mock.ExpectRollback()
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFinanceAggregationLockCancellationAndFailure(t *testing.T) {
	require.ErrorIs(t, AcquireFinanceAggregationLock(context.Background(), nil), service.ErrFinanceRecalculationSyncRequired)
	db, mock := newSQLMock(t)
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, AcquireFinanceAggregationLock(ctx, tx), context.Canceled)
	require.ErrorIs(t, SynchronizeFinanceCostAggregates(ctx, tx, nil, nil), context.Canceled)
	mock.ExpectExec(`LOCK TABLE usage_logs IN ROW SHARE MODE`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WithArgs(financeAggregationLockID).WillReturnError(context.DeadlineExceeded)
	require.ErrorIs(t, AcquireFinanceAggregationLock(context.Background(), tx), context.DeadlineExceeded)
	mock.ExpectRollback()
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

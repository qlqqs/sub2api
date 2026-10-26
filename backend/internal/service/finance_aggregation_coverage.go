package service

import infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"

// Historical membership is not inferred from survivors. Legacy repair requires
// explicit retained-baseline acceptance and successful full-day reconciliation.
// If either is missing, reject atomically and preserve the historical totals.
var ErrFinanceRecalculationCoverageUnknown = infraerrors.Conflict(
	"FINANCE_RECALCULATION_COVERAGE_UNKNOWN",
	"Source coverage is unproven or retained-detail baseline reconciliation failed; historical aggregates were preserved and no costs were changed",
)

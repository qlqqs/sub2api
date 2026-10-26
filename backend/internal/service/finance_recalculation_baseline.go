package service

// FinanceRetainedBaseline is an explicitly accepted, per-batch legacy baseline.
// It records retained source membership and the original reconciled aggregates;
// it does not claim that no source was deleted before this snapshot, and does
// not alter the global prospective aggregation coverage frontier.
// The audit is persisted only inside the whitelisted configuration snapshot.
type FinanceRetainedBaseline struct {
	DeletionGeneration int64                        `json:"deletion_generation"`
	Version            int                          `json:"version"`
	Timezone           string                       `json:"timezone"`
	Days               []FinanceRetainedBaselineDay `json:"days"`
}

const FinanceRetainedBaselineMaxDays = 31

type FinanceRetainedBaselineDay struct {
	Day          string                             `json:"day"`
	SourceRows   int64                              `json:"source_rows"`
	SourceDigest string                             `json:"source_digest"`
	Hourly       []FinanceRetainedBaselineAggregate `json:"hourly"`
	Daily        FinanceRetainedBaselineAggregate   `json:"daily"`
}

// Decimal strings retain the stored aggregate precision, without float rounding.
// Bucket is YYYY-MM-DD for daily and RFC3339 for hourly aggregates.
type FinanceRetainedBaselineAggregate struct {
	Bucket      string `json:"bucket"`
	Requests    int64  `json:"requests"`
	AccountCost string `json:"account_cost"`
	TotalCost   string `json:"total_cost"`
	ActualCost  string `json:"actual_cost"`
}

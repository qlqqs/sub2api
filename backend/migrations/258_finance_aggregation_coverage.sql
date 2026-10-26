-- Prospective proof of complete source coverage. Legacy snapshots contain no
-- source membership and cannot be certified from computed_at/count/MAX(id).
-- Never backdate this frontier in production. No usage rows are copied/rewritten.
CREATE TABLE IF NOT EXISTS finance_aggregation_coverage_state (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    complete_from TIMESTAMPTZ NOT NULL,
    initialized_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);


-- Only these buckets opt into preservation after source retention. Unmarked
-- legacy buckets retain existing periodic dashboard/recompute behavior.
CREATE TABLE IF NOT EXISTS finance_aggregation_repaired_hourly (
 bucket_start TIMESTAMPTZ PRIMARY KEY
);
CREATE TABLE IF NOT EXISTS finance_aggregation_repaired_daily (
 bucket_date DATE PRIMARY KEY
);


CREATE OR REPLACE FUNCTION finance_usage_coverage_before_delete()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    -- Same advisory key/order as AcquireFinanceAggregationLock, before row locks.
    PERFORM pg_advisory_xact_lock(8300804155680645479::bigint);
    RETURN NULL;
END $$;

CREATE OR REPLACE FUNCTION finance_usage_coverage_after_delete()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE frontier TIMESTAMPTZ;
BEGIN
    SELECT MAX(created_at) + INTERVAL '1 microsecond' INTO frontier FROM finance_deleted_usage;
    UPDATE finance_aggregation_coverage_state
       SET complete_from = frontier
     WHERE id = 1 AND complete_from < frontier;
    RETURN NULL;
END $$;

CREATE OR REPLACE FUNCTION finance_usage_coverage_before_truncate()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(8300804155680645479::bigint);
    UPDATE finance_aggregation_coverage_state
       SET complete_from = GREATEST(complete_from, clock_timestamp(),
           (SELECT MAX(created_at) + INTERVAL '1 microsecond' FROM usage_logs))
     WHERE id = 1;
    RETURN NULL;
END $$;

DROP TRIGGER IF EXISTS finance_usage_coverage_delete_lock ON usage_logs;
CREATE TRIGGER finance_usage_coverage_delete_lock BEFORE DELETE ON usage_logs
FOR EACH STATEMENT EXECUTE FUNCTION finance_usage_coverage_before_delete();
DROP TRIGGER IF EXISTS finance_usage_coverage_delete_frontier ON usage_logs;
CREATE TRIGGER finance_usage_coverage_delete_frontier AFTER DELETE ON usage_logs
REFERENCING OLD TABLE AS finance_deleted_usage FOR EACH STATEMENT
EXECUTE FUNCTION finance_usage_coverage_after_delete();
DROP TRIGGER IF EXISTS finance_usage_coverage_truncate ON usage_logs;
CREATE TRIGGER finance_usage_coverage_truncate BEFORE TRUNCATE ON usage_logs
FOR EACH STATEMENT EXECUTE FUNCTION finance_usage_coverage_before_truncate();

-- Initialize only AFTER trigger DDL has locked the source table for this
-- migration transaction. This closes the scan/install deletion race without a
-- separate long-running runtime table lock. Startup must not mix old workers
-- that do not participate in the advisory protocol with the new aggregation.
INSERT INTO finance_aggregation_coverage_state(id, complete_from)
SELECT 1, GREATEST(
    clock_timestamp(),
    (SELECT MAX(created_at) + INTERVAL '1 microsecond' FROM usage_logs),
    (SELECT MAX(bucket_start) + INTERVAL '1 hour' FROM usage_dashboard_hourly),
    -- Date-only legacy keys lack a timezone. +36h is an upper bound for the
    -- end of that calendar day in supported timezones, including UTC-12.
    (SELECT MAX(bucket_date)::timestamp AT TIME ZONE 'UTC' + INTERVAL '36 hours' FROM usage_dashboard_daily)
) ON CONFLICT (id) DO NOTHING;

-- CUSTOM(fork): accepted retained-detail baselines may predate the prospective
-- frontier. Keep normal aggregation live until a later source deletion. A
-- global generation deliberately invalidates conservatively (even deletions
-- outside a repaired day); it never backdates or claims complete old history.
ALTER TABLE finance_aggregation_coverage_state
    ADD COLUMN IF NOT EXISTS deletion_generation BIGINT NOT NULL DEFAULT 0;
ALTER TABLE finance_aggregation_repaired_hourly
    ADD COLUMN IF NOT EXISTS baseline_generation BIGINT;
ALTER TABLE finance_aggregation_repaired_daily
    ADD COLUMN IF NOT EXISTS baseline_generation BIGINT;

-- Replace functions forward-only; migration 258 remains immutable. The existing
-- BEFORE DELETE/TRUNCATE advisory lock serializes these generation changes with
-- aggregation and baseline validation. Ordinary inserts do not invalidate it.
CREATE OR REPLACE FUNCTION finance_usage_coverage_after_delete()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE frontier TIMESTAMPTZ;
BEGIN
    SELECT MAX(created_at) + INTERVAL '1 microsecond' INTO frontier FROM finance_deleted_usage;
    IF frontier IS NOT NULL THEN
        UPDATE finance_aggregation_coverage_state
           SET complete_from = GREATEST(complete_from, frontier),
               deletion_generation = deletion_generation + 1
         WHERE id = 1;
    END IF;
    RETURN NULL;
END $$;

CREATE OR REPLACE FUNCTION finance_usage_coverage_before_truncate()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(8300804155680645479::bigint);
    UPDATE finance_aggregation_coverage_state
       SET complete_from = GREATEST(complete_from, clock_timestamp(),
           (SELECT MAX(created_at) + INTERVAL '1 microsecond' FROM usage_logs)),
           deletion_generation = deletion_generation + 1
     WHERE id = 1;
    RETURN NULL;
END $$;

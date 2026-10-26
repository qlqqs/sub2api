-- CUSTOM(fork): bounded durable repair/audit jobs. No source FK: retention and
-- account/provider deletion must never erase an audit. No undo operation.
CREATE TABLE IF NOT EXISTS finance_recalculation_batches (
 id BIGSERIAL PRIMARY KEY,
 kind TEXT NOT NULL CHECK (kind IN ('usage_cost','payment_fee')),
 status TEXT NOT NULL DEFAULT 'previewing' CHECK (status IN ('previewing','ready','queued','applying','syncing','completed','failed','conflict','expired')),
 actor_id BIGINT NOT NULL CHECK (actor_id > 0),
 idempotency_key VARCHAR(128) NOT NULL,
 filter JSONB NOT NULL,
 start_at TIMESTAMPTZ NOT NULL,
 end_at TIMESTAMPTZ NOT NULL,
 cutoff_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 config_fingerprint TEXT NOT NULL DEFAULT '',
 config_snapshot JSONB NOT NULL DEFAULT '{}',
 scanned INTEGER NOT NULL DEFAULT 0 CHECK (scanned BETWEEN 0 AND 2000),
 changed INTEGER NOT NULL DEFAULT 0,
 unchanged INTEGER NOT NULL DEFAULT 0,
 skipped INTEGER NOT NULL DEFAULT 0,
 old_total NUMERIC NOT NULL DEFAULT 0,
 new_total NUMERIC NOT NULL DEFAULT 0,
 delta NUMERIC NOT NULL DEFAULT 0,
 lease_token TEXT NOT NULL DEFAULT '',
 lease_until TIMESTAMPTZ,
 failed_phase TEXT NOT NULL DEFAULT '',
 error_code TEXT NOT NULL DEFAULT '',
 error_message TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 expires_at TIMESTAMPTZ NOT NULL DEFAULT (clock_timestamp() + INTERVAL '30 minutes'),
 confirmed_at TIMESTAMPTZ,
 completed_at TIMESTAMPTZ,
 UNIQUE(actor_id,idempotency_key),
 CHECK (start_at < end_at),
 CHECK (changed >= 0 AND unchanged >= 0 AND skipped >= 0 AND scanned=changed+unchanged+skipped)
);
CREATE INDEX IF NOT EXISTS finance_recalculation_worker_idx ON finance_recalculation_batches(status,lease_until,id)
 WHERE status IN ('previewing','queued','applying','syncing');
CREATE TABLE IF NOT EXISTS finance_recalculation_items (
 id BIGSERIAL PRIMARY KEY,
 batch_id BIGINT NOT NULL REFERENCES finance_recalculation_batches(id),
 target_id BIGINT NOT NULL,
 target_created_at TIMESTAMPTZ NOT NULL,
 source_fingerprint TEXT NOT NULL,
 status TEXT NOT NULL CHECK (status IN ('changed','unchanged','skipped')),
 reason TEXT NOT NULL DEFAULT '',
 source TEXT NOT NULL DEFAULT '',
 old_cost NUMERIC,
 new_cost NUMERIC,
 delta NUMERIC,
 candidate JSONB NOT NULL,
 usage_result JSONB,
 payment_result JSONB,
 applied BOOLEAN NOT NULL DEFAULT FALSE,
 applied_at TIMESTAMPTZ,
 UNIQUE(batch_id,target_id,target_created_at)
);
CREATE INDEX IF NOT EXISTS finance_recalculation_items_page_idx ON finance_recalculation_items(batch_id,id);

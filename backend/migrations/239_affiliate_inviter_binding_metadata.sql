-- Persist the one-time inviter binding boundary separately from the current FK.
ALTER TABLE user_affiliates
    ADD COLUMN IF NOT EXISTS inviter_bound_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS inviter_bound_source VARCHAR(32),
    ADD COLUMN IF NOT EXISTS inviter_bound_by BIGINT,
    ADD COLUMN IF NOT EXISTS inviter_bind_reason VARCHAR(500);

-- Add the FK separately so a retry after a partially applied migration also
-- repairs a column that exists without its constraint.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'user_affiliates_inviter_bound_by_fkey'
          AND conrelid = 'user_affiliates'::regclass
    ) THEN
        ALTER TABLE user_affiliates
            ADD CONSTRAINT user_affiliates_inviter_bound_by_fkey
            FOREIGN KEY (inviter_bound_by) REFERENCES users(id) ON DELETE SET NULL;
    END IF;
END
$$;

-- Existing relationships were created during registration; preserve their
-- previous effective-date behavior by using the affiliate profile timestamp.
UPDATE user_affiliates
SET inviter_bound_at = COALESCE(inviter_bound_at, created_at),
    inviter_bound_source = COALESCE(inviter_bound_source, 'registration')
WHERE inviter_id IS NOT NULL
  AND (inviter_bound_at IS NULL OR inviter_bound_source IS NULL);

ALTER TABLE user_affiliates
    DROP CONSTRAINT IF EXISTS user_affiliates_inviter_bound_source_check;
ALTER TABLE user_affiliates
    ADD CONSTRAINT user_affiliates_inviter_bound_source_check
    CHECK (inviter_bound_source IS NULL OR inviter_bound_source IN ('registration', 'admin'));

-- Keep the permanent binding marker correct while old application binaries are
-- still able to write inviter_id without the new metadata columns. The trigger
-- also protects the marker from being cleared and prevents reassignment after
-- a relationship has ever been established. Setting inviter_id to NULL remains
-- allowed so the users FK can continue to apply ON DELETE SET NULL.
CREATE OR REPLACE FUNCTION enforce_user_affiliate_inviter_binding()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.inviter_id IS NOT NULL THEN
            NEW.inviter_bound_at := COALESCE(NEW.inviter_bound_at, NOW());
            NEW.inviter_bound_source := COALESCE(NEW.inviter_bound_source, 'registration');
        END IF;
        RETURN NEW;
    END IF;

    IF OLD.inviter_bound_at IS NOT NULL THEN
        -- The timestamp is the permanent one-time-binding marker. It must
        -- survive ordinary updates and the FK's ON DELETE SET NULL update.
        NEW.inviter_bound_at := OLD.inviter_bound_at;

        IF NEW.inviter_id IS NOT NULL
           AND NEW.inviter_id IS DISTINCT FROM OLD.inviter_id THEN
            RAISE EXCEPTION 'affiliate inviter has already been bound for user %', OLD.user_id
                USING ERRCODE = '23514';
        END IF;

        RETURN NEW;
    END IF;

    -- Legacy application versions update only inviter_id. Fill the metadata
    -- exactly when an unbound row transitions from NULL to a real inviter.
    IF OLD.inviter_id IS NULL AND NEW.inviter_id IS NOT NULL THEN
        NEW.inviter_bound_at := COALESCE(NEW.inviter_bound_at, NOW());
        NEW.inviter_bound_source := COALESCE(NEW.inviter_bound_source, 'registration');
    END IF;

    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS user_affiliates_inviter_binding_guard ON user_affiliates;
CREATE TRIGGER user_affiliates_inviter_binding_guard
BEFORE INSERT OR UPDATE OF inviter_id, inviter_bound_at, inviter_bound_source
ON user_affiliates
FOR EACH ROW
EXECUTE FUNCTION enforce_user_affiliate_inviter_binding();

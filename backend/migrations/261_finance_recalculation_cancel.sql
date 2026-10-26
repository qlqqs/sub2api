-- CUSTOM(fork): an unconfirmed preview can be discarded by its creator. Only the
-- status vocabulary changes; cancelled batches keep their audit rows and never
-- touch costs. Applied, syncing or completed batches still cannot be cancelled.
ALTER TABLE finance_recalculation_batches DROP CONSTRAINT IF EXISTS finance_recalculation_batches_status_check;
ALTER TABLE finance_recalculation_batches ADD CONSTRAINT finance_recalculation_batches_status_check
 CHECK (status IN ('previewing','ready','queued','applying','syncing','completed','failed','conflict','expired','cancelled'));

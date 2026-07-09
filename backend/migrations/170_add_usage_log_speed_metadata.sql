-- Persist speed-routing metadata already written by the usage log repository.
ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS speed_state TEXT,
    ADD COLUMN IF NOT EXISTS speed_wait_ms INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS speed_route TEXT;

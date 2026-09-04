ALTER TABLE outbox_events
 ADD COLUMN IF NOT EXISTS headers jsonb NOT NULL DEFAULT '{}'::jsonb;

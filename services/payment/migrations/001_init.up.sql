CREATE TABLE IF NOT EXISTS payments.payments (
 id uuid PRIMARY KEY, order_id uuid NOT NULL UNIQUE, user_id uuid NOT NULL,
 amount_cents bigint NOT NULL CHECK(amount_cents >= 0), currency char(3) NOT NULL,
 provider text NOT NULL DEFAULT 'mock', status text NOT NULL CHECK(status IN ('pending','succeeded','failed')),
 failure_reason text, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS payments.outbox_events (
 id uuid PRIMARY KEY, event_type text NOT NULL, payload jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), published_at timestamptz
);
CREATE TABLE IF NOT EXISTS payments.inbox_events (id uuid PRIMARY KEY, processed_at timestamptz NOT NULL DEFAULT now());
CREATE INDEX IF NOT EXISTS idx_payments_user ON payments.payments(user_id,created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payment_outbox ON payments.outbox_events(created_at) WHERE published_at IS NULL;

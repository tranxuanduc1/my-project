CREATE TABLE IF NOT EXISTS orders.products (
 id uuid PRIMARY KEY, sku text NOT NULL UNIQUE, name text NOT NULL, description text NOT NULL DEFAULT '',
 price_cents bigint NOT NULL CHECK(price_cents >= 0), currency char(3) NOT NULL DEFAULT 'USD',
 stock integer NOT NULL CHECK(stock >= 0), image_object_key text, active boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS orders.orders (
 id uuid PRIMARY KEY, user_id uuid NOT NULL, status text NOT NULL CHECK(status IN ('pending_payment','confirmed','payment_failed','cancelled')),
 amount_cents bigint NOT NULL CHECK(amount_cents >= 0), currency char(3) NOT NULL, idempotency_key text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(user_id,idempotency_key)
);
CREATE TABLE IF NOT EXISTS orders.order_items (
 id uuid PRIMARY KEY, order_id uuid NOT NULL REFERENCES orders.orders(id) ON DELETE CASCADE,
 product_id uuid NOT NULL, sku text NOT NULL, name text NOT NULL, unit_price_cents bigint NOT NULL,
 quantity integer NOT NULL CHECK(quantity > 0), line_total_cents bigint NOT NULL
);
CREATE TABLE IF NOT EXISTS orders.outbox_events (
 id uuid PRIMARY KEY, event_type text NOT NULL, payload jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), published_at timestamptz
);
CREATE TABLE IF NOT EXISTS orders.inbox_events (id uuid PRIMARY KEY, processed_at timestamptz NOT NULL DEFAULT now());
CREATE INDEX IF NOT EXISTS idx_orders_user ON orders.orders(user_id,created_at DESC);
CREATE INDEX IF NOT EXISTS idx_order_items_order ON orders.order_items(order_id);
CREATE INDEX IF NOT EXISTS idx_outbox_pending ON orders.outbox_events(created_at) WHERE published_at IS NULL;

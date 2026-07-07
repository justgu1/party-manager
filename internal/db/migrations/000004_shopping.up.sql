-- Shopping list: each item is a purchase paid by one user (with a mandatory
-- receipt). Costs are split equally among all users (balance = paid - share).
CREATE TABLE shopping_items (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         TEXT NOT NULL,
    unit_cents   BIGINT NOT NULL DEFAULT 0,
    quantity     INT NOT NULL DEFAULT 1,
    receipt_path TEXT NOT NULL DEFAULT '',
    paid_by      UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_shopping_paid_by ON shopping_items(paid_by);

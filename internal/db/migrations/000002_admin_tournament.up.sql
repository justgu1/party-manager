-- Admin flag ------------------------------------------------------------
ALTER TABLE users ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT FALSE;

-- Up/down votes: value is +1 (up) or -1 (down) -------------------------
ALTER TABLE rental_votes ADD COLUMN value SMALLINT NOT NULL DEFAULT 1;

-- Password reset tokens (used by the admin seeder + "forgot password") --
CREATE TABLE password_resets (
    token      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    used       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_password_resets_user ON password_resets(user_id);

-- Tournament singleton: continuous ranking by net score; capacity is the
-- number of places still advancing (16 -> 8 -> 4 -> 2 -> 1).
CREATE TABLE tournament (
    id         INT PRIMARY KEY DEFAULT 1,
    active     BOOLEAN NOT NULL DEFAULT FALSE,
    capacity   INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT tournament_singleton CHECK (id = 1)
);
INSERT INTO tournament (id, active, capacity) VALUES (1, FALSE, 0)
ON CONFLICT (id) DO NOTHING;

-- Manual detail fields for places (admin can enrich beyond scraping) ----
ALTER TABLE rentals ADD COLUMN notes TEXT NOT NULL DEFAULT '';

-- History of raffle/game draws.
CREATE TABLE game_draws (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mode         TEXT NOT NULL,
    options      JSONB NOT NULL,
    winner       TEXT NOT NULL,
    winner_index INT NOT NULL,
    created_by   UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_game_draws_created ON game_draws(created_at DESC);

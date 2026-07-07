-- Single-elimination bracket: each match pits two places; the higher net
-- score advances. Round 0 is the first round; the deepest round is the final.
CREATE TABLE bracket_matches (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    round      INT NOT NULL,
    position   INT NOT NULL,
    place_a    UUID REFERENCES rentals(id) ON DELETE SET NULL,
    place_b    UUID REFERENCES rentals(id) ON DELETE SET NULL,
    winner     UUID REFERENCES rentals(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_bracket_round ON bracket_matches(round, position);

ALTER TABLE tournament ADD COLUMN round  INT NOT NULL DEFAULT 0;
ALTER TABLE tournament ADD COLUMN rounds INT NOT NULL DEFAULT 0;

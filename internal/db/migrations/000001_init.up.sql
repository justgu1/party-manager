-- Users -----------------------------------------------------------------
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    name          TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Rentals (Part 1) -------------------------------------------------------
CREATE TABLE rentals (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    url           TEXT NOT NULL,
    source        TEXT NOT NULL DEFAULT 'generic', -- airbnb|booking|br|generic
    title         TEXT NOT NULL DEFAULT '',
    description   TEXT NOT NULL DEFAULT '',
    price         TEXT NOT NULL DEFAULT '',
    rating        TEXT NOT NULL DEFAULT '',
    reviews_count INTEGER NOT NULL DEFAULT 0,
    image_url     TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'pending',  -- pending|scraped|failed
    error         TEXT NOT NULL DEFAULT '',
    submitted_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE rental_availability (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rental_id UUID NOT NULL REFERENCES rentals(id) ON DELETE CASCADE,
    label     TEXT NOT NULL DEFAULT '',
    date_from DATE,
    date_to   DATE,
    raw       JSONB
);
CREATE INDEX idx_rental_availability_rental ON rental_availability(rental_id);

CREATE TABLE rental_votes (
    rental_id  UUID NOT NULL REFERENCES rentals(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (rental_id, user_id)
);

-- Prendas + Jukebox (Part 2) --------------------------------------------
CREATE TABLE prendas (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE songs (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    youtube_id    TEXT NOT NULL,
    url           TEXT NOT NULL,
    title         TEXT NOT NULL DEFAULT '',
    thumbnail_url TEXT NOT NULL DEFAULT '',
    author        TEXT NOT NULL DEFAULT '',
    requested_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    prenda_id     UUID REFERENCES prendas(id) ON DELETE SET NULL,
    prenda_done   BOOLEAN NOT NULL DEFAULT FALSE,
    status        TEXT NOT NULL DEFAULT 'queued', -- queued|playing|played
    position      BIGSERIAL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_songs_status ON songs(status);

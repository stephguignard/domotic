-- +goose Up

-- Équipement unifié, toutes sources confondues. L'identifiant est celui de la
-- source d'origine (URL Overkiz pour TaHoma, identifiant de module Netatmo) :
-- il est déjà unique et stable, inutile d'en générer un autre.
CREATE TABLE device (
    id          TEXT PRIMARY KEY,
    source      TEXT NOT NULL CHECK (source IN ('netatmo', 'tahoma')),
    name        TEXT NOT NULL,
    kind        TEXT NOT NULL,
    room        TEXT NOT NULL DEFAULT '',
    -- État courant normalisé, sérialisé en JSON : la forme varie trop d'un
    -- type d'équipement à l'autre pour tenir dans des colonnes fixes.
    state       TEXT NOT NULL DEFAULT '{}',
    reachable   INTEGER NOT NULL DEFAULT 1,
    updated_at  TIMESTAMP NOT NULL
);

CREATE INDEX idx_device_source ON device (source);
CREATE INDEX idx_device_room ON device (room);

-- Historique des relevés de capteurs (Netatmo essentiellement).
CREATE TABLE measurement (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id   TEXT NOT NULL REFERENCES device (id) ON DELETE CASCADE,
    metric      TEXT NOT NULL,
    value       REAL NOT NULL,
    recorded_at TIMESTAMP NOT NULL,
    -- La source peut renvoyer plusieurs fois le même relevé : le polling
    -- tourne plus vite que la station ne produit des mesures.
    UNIQUE (device_id, metric, recorded_at)
);

CREATE INDEX idx_measurement_lookup ON measurement (device_id, metric, recorded_at DESC);

-- Jetons OAuth2. Netatmo fait tourner le refresh token à chaque
-- rafraîchissement : cette table doit être mise à jour à chaque rotation,
-- sinon l'accès est perdu au prochain redémarrage.
CREATE TABLE oauth_token (
    provider      TEXT PRIMARY KEY,
    access_token  TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    token_type    TEXT NOT NULL DEFAULT 'Bearer',
    expiry        TIMESTAMP NOT NULL,
    updated_at    TIMESTAMP NOT NULL
);

-- +goose Down
DROP TABLE oauth_token;
DROP TABLE measurement;
DROP TABLE device;

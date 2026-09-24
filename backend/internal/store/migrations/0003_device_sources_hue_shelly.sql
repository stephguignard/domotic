-- +goose NO TRANSACTION
-- +goose Up

-- Élargit la contrainte de source à Philips Hue et Shelly.
--
-- SQLite ne sait pas modifier une contrainte CHECK : la table doit être
-- reconstruite. Supprimer l'ancienne table avec les clés étrangères actives
-- déclencherait l'ON DELETE CASCADE de `measurement` et effacerait tout
-- l'historique. Or PRAGMA foreign_keys est sans effet à l'intérieur d'une
-- transaction, d'où NO TRANSACTION et une transaction explicite, ouverte après
-- la désactivation. Le pool de connexions étant limité à une seule (store.Open),
-- toutes ces instructions passent par la même connexion.
PRAGMA foreign_keys = OFF;

BEGIN;

CREATE TABLE device_new (
    id          TEXT PRIMARY KEY,
    source      TEXT NOT NULL CHECK (source IN ('netatmo', 'tahoma', 'hue', 'shelly')),
    name        TEXT NOT NULL,
    kind        TEXT NOT NULL,
    room        TEXT NOT NULL DEFAULT '',
    state       TEXT NOT NULL DEFAULT '{}',
    reachable   INTEGER NOT NULL DEFAULT 1,
    updated_at  TIMESTAMP NOT NULL
);

INSERT INTO device_new (id, source, name, kind, room, state, reachable, updated_at)
SELECT id, source, name, kind, room, state, reachable, updated_at FROM device;

DROP TABLE device;
ALTER TABLE device_new RENAME TO device;

CREATE INDEX idx_device_source ON device (source);
CREATE INDEX idx_device_room ON device (room);

COMMIT;

PRAGMA foreign_keys = ON;

-- +goose Down

PRAGMA foreign_keys = OFF;

BEGIN;

-- L'ancienne contrainte refuserait ces lignes : elles sont perdues au retour
-- arrière, avec leurs relevés.
DELETE FROM measurement WHERE device_id IN (SELECT id FROM device WHERE source IN ('hue', 'shelly'));
DELETE FROM device WHERE source IN ('hue', 'shelly');

CREATE TABLE device_old (
    id          TEXT PRIMARY KEY,
    source      TEXT NOT NULL CHECK (source IN ('netatmo', 'tahoma')),
    name        TEXT NOT NULL,
    kind        TEXT NOT NULL,
    room        TEXT NOT NULL DEFAULT '',
    state       TEXT NOT NULL DEFAULT '{}',
    reachable   INTEGER NOT NULL DEFAULT 1,
    updated_at  TIMESTAMP NOT NULL
);

INSERT INTO device_old (id, source, name, kind, room, state, reachable, updated_at)
SELECT id, source, name, kind, room, state, reachable, updated_at FROM device;

DROP TABLE device;
ALTER TABLE device_old RENAME TO device;

CREATE INDEX idx_device_source ON device (source);
CREATE INDEX idx_device_room ON device (room);

COMMIT;

PRAGMA foreign_keys = ON;

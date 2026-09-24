-- +goose Up

-- Scènes : une suite d'étapes (actions et attentes) et les horaires qui la
-- déclenchent. Étapes et horaires sont stockés en JSON : leur forme varie d'une
-- étape à l'autre, et ils ne sont jamais interrogés isolément.
CREATE TABLE scene (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    name               TEXT NOT NULL,
    show_on_dashboard  INTEGER NOT NULL DEFAULT 0,
    steps              TEXT NOT NULL DEFAULT '[]',
    schedules          TEXT NOT NULL DEFAULT '[]',
    -- Dernière exécution : début, déclencheur (manual, schedule) et issue
    -- (running, success, partial, failed, interrupted).
    last_run_at        TIMESTAMP,
    last_trigger       TEXT NOT NULL DEFAULT '',
    last_status        TEXT NOT NULL DEFAULT '',
    created_at         TIMESTAMP NOT NULL,
    updated_at         TIMESTAMP NOT NULL
);

-- +goose Down
DROP TABLE scene;

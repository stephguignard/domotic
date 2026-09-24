-- +goose Up

-- Historique des commandes envoyées depuis l'interface, réussies ou non.
--
-- Pas de clé étrangère vers device : l'historique doit survivre à la
-- disparition d'un équipement (renommé côté source, module retiré). Le nom et
-- la source sont donc copiés au moment de l'action, tels qu'ils étaient alors.
CREATE TABLE command_log (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id    TEXT NOT NULL,
    device_name  TEXT NOT NULL,
    device_kind  TEXT NOT NULL,
    source       TEXT NOT NULL,
    command      TEXT NOT NULL,
    -- Paramètres de la commande, sérialisés en JSON (tableau).
    parameters   TEXT NOT NULL DEFAULT '[]',
    success      INTEGER NOT NULL,
    error        TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMP NOT NULL
);

CREATE INDEX idx_command_log_time ON command_log (created_at DESC);
CREATE INDEX idx_command_log_device ON command_log (device_id, created_at DESC);

-- +goose Down
DROP TABLE command_log;

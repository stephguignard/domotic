-- +goose Up

-- Origine des actions de l'historique. Les entrées antérieures viennent toutes
-- de l'interface. scene_name est copié, comme device_name, pour survivre à la
-- suppression de la scène.
ALTER TABLE command_log ADD COLUMN origin TEXT NOT NULL DEFAULT 'interface';
ALTER TABLE command_log ADD COLUMN scene_id INTEGER;
ALTER TABLE command_log ADD COLUMN scene_name TEXT NOT NULL DEFAULT '';

CREATE INDEX idx_command_log_scene ON command_log (scene_id, created_at DESC);

-- +goose Down
DROP INDEX idx_command_log_scene;
ALTER TABLE command_log DROP COLUMN scene_name;
ALTER TABLE command_log DROP COLUMN scene_id;
ALTER TABLE command_log DROP COLUMN origin;

-- +goose Up

-- Ordre d'affichage des scènes choisi dans l'interface. Une nouvelle scène
-- prend la dernière place ; NULL (scènes antérieures, ou ordre remis à zéro)
-- range la scène après les autres, par ordre alphabétique.
ALTER TABLE scene ADD COLUMN position INTEGER;

-- +goose Down
ALTER TABLE scene DROP COLUMN position;

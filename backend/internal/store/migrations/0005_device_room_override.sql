-- +goose Up

-- Pièce choisie dans l'interface, prioritaire sur celle de la source.
--
-- Colonne distincte de `room` : le polling réécrit `room` à chaque cycle avec
-- ce que fournit la source, et effacerait un choix fait à la main. NULL signifie
-- « suivre la source » ; une chaîne vide, « sans pièce » par choix.
ALTER TABLE device ADD COLUMN room_override TEXT;

-- +goose Down
ALTER TABLE device DROP COLUMN room_override;

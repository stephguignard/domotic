-- +goose Up

-- Ordre d'affichage des pièces choisi dans l'interface. Une pièce absente de
-- la table n'a pas été classée : elle s'affiche après les autres, par ordre
-- alphabétique. Une pièce classée puis vidée garde sa position, qu'elle
-- retrouve si on y range de nouveau un équipement.
CREATE TABLE room_order (
    room      TEXT PRIMARY KEY,
    position  INTEGER NOT NULL
);

-- +goose Down
DROP TABLE room_order;

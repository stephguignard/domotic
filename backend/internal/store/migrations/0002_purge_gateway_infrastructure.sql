-- +goose Up

-- Nettoyage ponctuel : les passerelles de protocole et composants internes de
-- la box TaHoma ont été enregistrés comme des équipements avant que le client
-- ne les écarte. UpsertDevices ne supprimant jamais de ligne, ils resteraient
-- indéfiniment en base et continueraient de polluer l'interface.
--
-- Le filtrage vit désormais dans internal/tahoma (isInfrastructure) et porte
-- sur le controllableName. Celui-ci n'étant pas stocké, ce nettoyage ne peut
-- s'appuyer que sur le préfixe de protocole du deviceURL. Un équipement Zigbee
-- légitime qui serait effacé ici est réinséré au cycle de polling suivant,
-- puisque le filtrage du client ne l'écarte pas.
DELETE FROM device
WHERE source = 'tahoma'
  AND (
    id LIKE 'internal://%'
    OR id LIKE 'ogp://%'
    OR id LIKE 'zigbee://%'
  );

-- +goose Down

-- Rien à défaire : ces lignes sont reconstruites par le polling si le filtrage
-- du client est retiré.
SELECT 1;

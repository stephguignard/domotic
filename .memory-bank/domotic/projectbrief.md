# Project Brief — domotic

## Ce que c'est

Un service d'agrégation domotique **personnel** : un binaire Go unique consolide
les équipements **Netatmo** (cloud, OAuth2) et **Somfy TaHoma** (API locale sur le
LAN) derrière une API REST unifiée, et sert lui-même un frontend Angular embarqué.

## Contrainte fondatrice

La cible est un **Synology DS216+** — Celeron N3050, **1 Go de RAM**, DSM 7.1 — où
Home Assistant ne tient pas. Toute dépendance ou service supplémentaire se juge à
l'aune de cette contrainte (voir [[decisions]] #1).

## Périmètre

- Dans le périmètre : lecture consolidée des équipements et mesures, commandes
  TaHoma, historique des mesures, interface web légère, déploiement Docker sur le NAS.
- Hors périmètre (pour l'instant) : multi-utilisateur, authentification de l'UI,
  automatisations/scénarios, accès depuis Internet.

## Critères de réussite

- Tourne en continu sur le NAS avec une empreinte de quelques dizaines de Mo.
- Survit aux redémarrages sans ré-authentification Netatmo.
- L'interface reste réactive quand une source amont est indisponible.
- `make test` et `make lint` passent.

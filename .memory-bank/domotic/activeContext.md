# Active Context — domotic

_Dernière mise à jour : 2026-09-23 (clôture de la session d'intégration TaHoma)_

## Branche courante

`main`, arbre propre. `chore/memoire-projet` y a été fusionnée en fast-forward
(mémoire de projet : Serena, memory-bank, clôture de session) ; l'historique reste
linéaire et la branche peut être supprimée.

## Récemment fait

- **TaHoma validé en réel** : box à l'IP fixe, quatre volets remontés, une commande
  `close` a physiquement fermé un volet. Chaîne complète éprouvée, TLS compris.
- Passerelles internes écartées sur le `controllableName` (11 équipements → 4) +
  migration `0002` pour purger les lignes déjà enregistrées.
- `.env` désormais chargé par le Makefile ; deux tests de config qui dépendaient de
  l'environnement ont dû être isolés.
- Absence de configuration Netatmo expliquée par une page dédiée (503) au lieu de
  retomber silencieusement sur l'application.
- SCSS → CSS + Tailwind 4, puis styles réécrits en classes utilitaires : 400 lignes
  de CSS ramenées à 92, plus aucun `::ng-deep`.
- **Docker éprouvé en local** : image 17,5 Mo, 6,8 Mo de RAM au repos, et le
  conteneur atteint la box sur le LAN par le bridge par défaut.
- Serena + memory-bank, commandes `/hello` et `/bye`.

## Décisions ouvertes

- Pièces TaHoma : l'API locale n'en fournit aucune → groupement par type, ou mapping
  manuel en configuration ? Rien n'est engagé.
- Retour d'état des volets RTS : récepteur ESP32 + CC1101 exposant du MQTT (idée
  seulement).
- Robustesse de l'arrêt de `make dev` : le `trap` ne tue pas `ng serve` quand
  l'arrêt vient d'un signal plutôt que d'un `Ctrl+C`.

## Attention

- État de volet vide et « Sans pièce » = normal, pas des bugs (RTS + API locale).
- **Netatmo jamais connecté** : il manque `client_id` / `client_secret`. Le test réel
  de la rotation du refresh token (auth → redémarrage → appel suivant) reste à faire ;
  c'est le seul qui valide vraiment le mécanisme.
- L'image Docker construite contient encore le frontend d'avant Tailwind :
  `make docker` avant tout déploiement.
- `make dev` et le conteneur se disputent le port 8080 : n'en lancer qu'un.

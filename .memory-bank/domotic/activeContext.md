# Active Context — domotic

_Dernière mise à jour : 2026-09-23 (mise en place de Serena et du memory-bank)_

## Branche courante

`main`, HEAD `526e3fe` (« Écrit les styles en classes Tailwind »). Pas de travail en cours
sur une autre branche.

## Récemment fait

- Passage de SCSS à CSS + Tailwind 4, styles réécrits en classes utilitaires.
- Écart des passerelles internes TaHoma (`ogp://`, `internal://`, `zigbee://`),
  migration `0002_purge_gateway_infrastructure.sql`.
- Absence de configuration Netatmo expliquée dans l'UI au lieu d'être tue.
- Serena (Go + TypeScript) et memory-bank ajoutés, commandes `/hello` et `/bye`.

## Décisions ouvertes

- Pièces TaHoma : l'API locale n'en fournit pas → mapping manuel en configuration ?
- Retour d'état des volets RTS : récepteur ESP32 + CC1101 / MQTT (non engagé).

## Attention

- État de volet vide = normal (RTS), pas un bug.
- Netatmo non connecté : le test réel de la rotation du refresh token
  (auth → redémarrage → appel) reste à faire.

# Product Context — domotic

## Problème

Deux écosystèmes (Netatmo, Somfy TaHoma) avec chacun leur application, et pas de
vue unifiée. Les solutions génériques (Home Assistant) sont trop lourdes pour le NAS.

## Utilisateur

Un seul : le propriétaire de l'installation, sur le réseau local.

## Installation réelle (vérifiée le 2026-09-23)

- Box TaHoma en mode développeur (API locale, port 8443) exposant **quatre volets
  RTS** : Bureau, Chambre parent, Salon grande porte, Salon petite porte.
- Le **RTS est unidirectionnel** : aucun retour d'état (`states=0`), une commande
  acceptée ne prouve pas que le volet a bougé. Ne pas promettre de position.
- L'API locale n'expose **aucune pièce** (ni `rootPlace` ni `placeOID`).
- Les passerelles de protocole internes (`ogp://`, `internal://`, `zigbee://`) ne
  sont pas des équipements : elles sont écartées (migration `0002`).
- **Netatmo pas encore connecté.**

## Expérience visée

- Tableau de bord : vue d'ensemble, équipements injoignables mis en évidence.
- Liste des équipements groupés par pièce, fiche détail avec historique (Chart.js).
- Commandes de volets immédiates (ouvrir / fermer / stop).

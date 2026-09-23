# Progress — domotic

_Dernier état connu : 2026-09-23, mesuré à la clôture précédente (HEAD `714d664`)._
_Inchangé depuis : la session suivante n'a touché qu'à l'outillage Git._

## Build / tests

- `make test` : **vert** — 4 paquets Go (`config`, `store`, `tahoma`, `web`) et
  56 tests Vitest sur 6 fichiers.
- `make lint` : **vert** (`go vet` + vérification `gofmt`).
- `make build` : binaire ~15 Mo, frontend embarqué.
- `make docker` : image **17,5 Mo**, **6,8 Mo de RAM** au repos sous `mem_limit: 128m`
  — large marge sur le gigaoctet du DS216+.

## Par composant

| Composant | État |
|---|---|
| Config / store / migrations | ✅ fonctionnels, testés |
| TaHoma (client, events, commandes) | ✅ **validé sur matériel réel** : lecture, TLS, commande exécutée |
| Netatmo (OAuth2, rotation jeton) | 🟡 implémenté et testé unitairement, **jamais connecté** |
| Poller | ✅ |
| API REST + OpenAPI + client généré | ✅ |
| Frontend (dashboard, liste, détail) | ✅ testé (Vitest), rendu vérifié par capture |
| Docker (image, volume, accès LAN) | ✅ éprouvé en local |
| Déploiement NAS | 🟡 chaîne prête (`make deploy`), jamais exécutée sur le NAS |
| Outillage Git | ✅ `gh` installé et authentifié, flux branches + PR en place |

## Problèmes connus

- **Aucune pièce côté TaHoma** : `/setup` ne renvoie ni `rootPlace` ni `placeOID`
  (cloud-only) → tout tombe dans « Sans pièce ».
- **Volets RTS sans état** : protocole unidirectionnel, `states: []` en permanence.
  Une commande acceptée ne prouve pas que le volet a bougé.
- L'arrêt de `make dev` par signal laisse `ng serve` orphelin (`Ctrl+C` le tue bien).
- Image Docker en retard d'un build : contient le frontend d'avant Tailwind.
- Deux conventions de commit coexistent dans le dépôt (voir `decisions.md` #14).

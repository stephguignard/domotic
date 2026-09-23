# Core — domotic

Agrégateur domotique perso : binaire Go (Huma + SQLite) consolidant Netatmo (cloud,
OAuth2) et TaHoma (API locale LAN), frontend Angular 22 embarqué. Cible : Synology
DS216+, 1 Go RAM → toute dépendance/service se justifie.

## Carte des sources
- `backend/cmd/domotic/` — cobra : `serve` (défaut), `openapi`.
- `backend/internal/config/` — env, `validate()` : intégration absente = inerte,
  partielle = refus de démarrer.
- `backend/internal/store/` — SQLite `modernc.org/sqlite`, migrations goose
  `migrations/NNNN_*.sql` (embarquées), jetons, mesures, équipements.
- `backend/internal/api/` — opérations Huma (`devices`, `measurements`, `health`),
  `auth.go` = flux OAuth2 Netatmo hors Huma (`RegisterAuthRoutes`).
- `backend/internal/poller/` — boucles Netatmo / TaHoma setup+events / rétention.
- `backend/internal/netatmo/` — client + `tokensource.go` (rotation persistée).
- `backend/internal/tahoma/` — client TLS par IP, `events.go` (listener recyclé).
- `backend/web/` — embed `all:dist` + repli SPA + `placeholderHTML`.
- `frontend/src/app/core/` — `DevicesStore` (signaux), `device-state.ts` (mise en forme pure).
- `frontend/src/app/features/` — `dashboard`, `devices` (liste, détail + Chart.js).
- `frontend/src/app/api/` — **généré, git-ignoré, ne jamais éditer**.

## Invariants
- Handlers ne lisent que SQLite ; seule exception `POST /api/devices/{id}/command`.
- Changement de type/handler Go → `make api-client`.
- Volets RTS : état toujours vide, pas de pièces côté TaHoma — pas des bugs.
- Filtrage des passerelles TaHoma : sur `controllableName`, jamais sur le
  préfixe de `deviceURL`.

## Mémoires ciblées
- Versions, dépendances, contraintes de build : `mem:tech_stack`
- Commandes make / tests isolés : `mem:suggested_commands`
- Langue, style Go/Angular, contraintes OpenAPI : `mem:conventions`
- Définition de « terminé » : `mem:task_completion`
- Pièges Vitest/jsdom, fabriques de test : `mem:testing-setup`
- Sonder la box sans jeton, endpoints locaux, critère de filtrage : `mem:tahoma-diagnostics`

## Contexte hors code
Mémoire longue dans `.memory-bank/domotic/` (projectbrief, productContext,
systemPatterns, techContext, activeContext, progress, `journal.md` append-only,
`decisions.md` ADR). `/hello` la charge, `/bye` la met à jour.

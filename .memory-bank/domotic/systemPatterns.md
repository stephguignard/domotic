# System Patterns — domotic

## Flux de données

```
Netatmo (cloud) ──┐
                  ├──▶ poller ──▶ SQLite ──▶ API REST (Huma) ──▶ Angular
TaHoma (LAN)    ──┘
```

- `internal/poller` : boucles en tâche de fond (Netatmo périodique, TaHoma setup +
  flux d'événements, rétention des mesures). Statut par source exposé via `/api/health`.
- Les handlers `internal/api` **ne lisent que SQLite**. Seule exception :
  `POST /api/devices/{id}/command`, qui traverse vers la box.

## Contrat d'API descendant

`internal/api/*.go` (types annotés Huma) → `make api-client` → `api/openapi*.json`
→ `frontend/src/app/api/` (généré, git-ignoré, jamais édité).
- Tags d'opération en ASCII ; slices de sortie `nullable:"false"`.
- La spec 3.0.3 alimente `typescript-angular` (support 3.1 partiel).

## Backend

- `cmd/domotic` : cobra, sous-commandes `serve` (défaut) et `openapi`.
- `internal/store` : SQLite `modernc.org/sqlite` + migrations goose embarquées
  (`migrations/NNNN_*.sql`).
- `internal/netatmo/tokensource.go` : persiste chaque rotation du refresh token,
  **erreur** si l'écriture échoue.
- `internal/tahoma/client.go` : dial par IP, `ServerName: gateway-<pin>.local`,
  CA Overkiz embarquée. Jamais `InsecureSkipVerify`.
- `web/embed.go` : `//go:embed all:dist` + repli SPA ; `placeholderHTML` si le
  front n'est pas compilé.

## Frontend

- Angular 22 standalone, composants chargés en `loadComponent` (`app.routes.ts`),
  `withComponentInputBinding()` pour `devices/:id`.
- `core/devices.store.ts` : store `providedIn: 'root'`, signaux privés exposés en
  `asReadonly()` / `computed()`, rafraîchissement périodique (pas de WebSocket/SSE).
- `core/device-state.ts` : fonctions pures de mise en forme (`parseState`,
  `formatValue`, `commandsFor`, …).
- UI : `@openng/optimus-ui` + Tailwind 4 (classes utilitaires, plus de SCSS), primeicons.

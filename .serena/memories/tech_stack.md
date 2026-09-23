# Tech stack

- Go 1.27 dans `~/.local/go` (hors PATH ; le Makefile l'exporte).
- Huma v2 (OpenAPI 3.1 + export 3.0.3), cobra, goose v3, `golang.org/x/oauth2`,
  `modernc.org/sqlite` (Go pur).
- Angular 22 standalone + signaux, TypeScript 6, Tailwind 4 (`@tailwindcss/postcss`),
  `@openng/optimus-ui` (+ themes, + tailwindcss), primeicons, Chart.js 4.
- Client API : `openapi-generator-cli` `typescript-angular` (Java 17+).
- npm **12+** : npm 10 casse (`edgesOut`) ; scripts d'install bloqués → champ
  `allowScripts` de `frontend/package.json`, `npm install-scripts approve <pkg>`.

## Build / déploiement
- `CGO_ENABLED=0` obligatoire (distroless/static). Jamais `mattn/go-sqlite3`.
- Jamais `GOAMD64=v3` (Celeron N3050 sans AVX2).
- Image nonroot UID 65532 → `chown -R 65532:65532` du volume `/data`.

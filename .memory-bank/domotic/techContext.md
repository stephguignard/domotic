# Tech Context — domotic

## Versions

| Élément | Version |
|---|---|
| Go | 1.27 (dans `~/.local/go`, hors `PATH`) |
| Huma | v2.39 |
| goose | v3.28 |
| modernc.org/sqlite | v1.59 |
| Angular | 22.x |
| TypeScript | 6.0 |
| Tailwind | 4.3 (`@tailwindcss/postcss`) |
| UI | `@openng/optimus-ui` 2.x |
| Tests front | Vitest 4 (jsdom) |
| npm | **12+** obligatoire |
| Java | 17+ (openapi-generator-cli) |

## Pièges d'environnement

- `export PATH=$HOME/.local/go/bin:$PATH` dans un appel shell direct.
- npm 10 casse sur les peer deps d'Angular 22 ; npm 12 bloque les scripts
  d'installation → `allowScripts` dans `frontend/package.json`
  (`npm install-scripts approve <pkg>`).
- jsdom sans Canvas : `src/test-setup.ts` stube `getContext()`.

## Déploiement

- `CGO_ENABLED=0`, `GOAMD64=v1` (Braswell sans AVX2), image `distroless/static`
  nonroot (UID 65532 → `chown` du volume `/data`).
- DSM 7.1 sans Container Manager : `make deploy` = `docker save | ssh | docker-compose up`.
- Image ~17,5 Mo, ~5 Mo de RAM au repos (mesuré sur le squelette).

## Configuration

Variables d'environnement (`internal/config`), `.env` chargé par le Makefile.
Intégration absente = inerte ; configuration partielle = refus de démarrer.

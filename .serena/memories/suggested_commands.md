# Commandes

- `make dev` — backend :8080 + frontend :4200 (proxy `/api`, `/auth`, `/docs`).
  Arrêt par signal (hors `Ctrl+C`) : le `trap` ne tue pas `ng serve`,
  finir avec `pkill -f 'ng serve'`. Le conteneur Docker occupe le même :8080.
- `make dev-backend` / `make dev-frontend` — une seule partie.
- `make build` — front compilé puis embarqué dans `./domotic`.
- `make openapi` / `make api-client` — specs puis client TS.
- `make test` (= `test-backend` + `test-frontend`), `make lint` (go vet + gofmt), `make fmt`.
- `make docker`, `make deploy NAS_HOST=nas NAS_DIR=/volume1/docker/domotic`.

## Tests isolés
- `cd backend && go test ./internal/store/ -run TestTokenRoundTrip -v`
  (exporter `PATH=$HOME/.local/go/bin:$PATH` hors Makefile).
- `cd frontend && npx ng test --watch=false --filter "^parseState"` — `--filter` =
  regex sur noms de suites/tests. Pas de `--browsers`.

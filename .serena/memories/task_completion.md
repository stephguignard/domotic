# Définition de « terminé »

1. Type/handler Go modifié → `make api-client`, le front compile.
2. `make lint` propre (go vet + gofmt).
3. `make test` vert (go test + vitest).
4. Nouvelle logique → test associé (specs front : vrai `DevicesStore` + HTTP intercepté).
5. Changement touchant le build de prod → `make build` (voire `make docker`).
6. Documentation : `CLAUDE.md`/README si une règle ou une commande change ;
   memory-bank via `/bye`.
7. Ne pas commiter sans demande explicite.

# Orchestration du monorepo : le backend se construit avec Go, le frontend avec
# l'Angular CLI. Ce Makefile ne fait que les enchaîner dans le bon ordre, la
# dépendance clé étant la spécification OpenAPI, produite par le backend et
# consommée par le frontend.

SHELL := /bin/bash
.DEFAULT_GOAL := help

BACKEND  := backend
FRONTEND := frontend
API_DIR  := api

SPEC_31 := $(API_DIR)/openapi.json
SPEC_30 := $(API_DIR)/openapi-3.0.json

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
IMAGE   ?= domotic:latest

# Déploiement : hôte SSH du NAS et répertoire de la stack.
NAS_HOST ?= nas
NAS_DIR  ?= /volume1/docker/domotic

# Go est installé dans ~/.local/go, hors du PATH par défaut.
export PATH := $(HOME)/.local/go/bin:$(PATH)

# docker-compose lit .env nativement, mais pas make : sans cela, les
# identifiants des intégrations resteraient invisibles en développement.
# Seules les clés réellement présentes dans le fichier sont exportées.
ifneq (,$(wildcard .env))
-include .env
export $(shell sed -n 's/^\([A-Z_][A-Z0-9_]*\)=.*/\1/p' .env)
endif

.PHONY: help
help: ## Afficher cette aide
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

# --- Développement ---------------------------------------------------------

.PHONY: dev
dev: ## Lancer le backend et le frontend en parallèle
	@echo "Backend  : http://localhost:8080  (API, /docs)"
	@echo "Frontend : http://localhost:4200  (proxie /api vers le backend)"
	@trap 'kill 0' EXIT INT TERM; \
	 $(MAKE) dev-backend & \
	 $(MAKE) dev-frontend & \
	 wait

.PHONY: dev-backend
dev-backend: ## Lancer le backend seul
	cd $(BACKEND) && go run ./cmd/domotic --debug

.PHONY: dev-frontend
dev-frontend: ## Lancer le frontend seul
	cd $(FRONTEND) && npm start

# --- Contrat d'API ---------------------------------------------------------

.PHONY: openapi
openapi: ## Générer les spécifications OpenAPI 3.1 et 3.0
	@mkdir -p $(API_DIR)
	cd $(BACKEND) && go run ./cmd/domotic openapi -o ../$(SPEC_31)
	cd $(BACKEND) && go run ./cmd/domotic openapi --downgrade -o ../$(SPEC_30)

.PHONY: api-client
api-client: openapi ## Régénérer le client TypeScript depuis la spécification
	@rm -rf $(FRONTEND)/src/app/api
	npx --yes @openapitools/openapi-generator-cli generate \
		-i $(SPEC_30) \
		-g typescript-angular \
		-o $(FRONTEND)/src/app/api \
		--additional-properties=ngVersion=22.0.0,providedInRoot=true,withInterfaces=true,fileNaming=kebab-case \
		--skip-validate-spec
	@echo "Client régénéré dans $(FRONTEND)/src/app/api"

# --- Build -----------------------------------------------------------------

.PHONY: build
build: build-frontend build-backend ## Compiler le tout (frontend embarqué dans le binaire)

.PHONY: build-frontend
build-frontend: ## Compiler le frontend et le placer dans le backend
	cd $(FRONTEND) && npm run build
	@mkdir -p $(BACKEND)/web/dist
	# Vider sans supprimer .gitkeep : c'est lui qui garantit l'existence du
	# répertoire sur un dépôt fraîchement cloné, donc la compilation du
	# //go:embed du package web.
	@find $(BACKEND)/web/dist -mindepth 1 ! -name .gitkeep -delete
	@touch $(BACKEND)/web/dist/.gitkeep
	cp -r $(FRONTEND)/dist/frontend/browser/. $(BACKEND)/web/dist/

.PHONY: build-backend
build-backend: ## Compiler le binaire Go statique
	cd $(BACKEND) && CGO_ENABLED=0 go build -trimpath \
		-ldflags="-s -w -X main.version=$(VERSION)" \
		-o ../domotic ./cmd/domotic
	@ls -lh domotic

# --- Qualité ---------------------------------------------------------------

.PHONY: test
test: test-backend test-frontend ## Lancer tous les tests

.PHONY: test-backend
test-backend: ## Tests Go
	cd $(BACKEND) && go test ./...

.PHONY: test-frontend
test-frontend: ## Tests Angular (Vitest, environnement jsdom)
	cd $(FRONTEND) && npm test -- --watch=false

.PHONY: lint
lint: ## Analyse statique
	cd $(BACKEND) && go vet ./...
	cd $(BACKEND) && gofmt -l . | (! grep .) || (echo "fichiers mal formatés ci-dessus" && exit 1)

.PHONY: fmt
fmt: ## Formater le code Go
	cd $(BACKEND) && gofmt -w .

# --- Docker et déploiement -------------------------------------------------

.PHONY: docker
docker: ## Construire l'image Docker
	docker build -f $(BACKEND)/Dockerfile --build-arg VERSION=$(VERSION) -t $(IMAGE) .
	@docker images $(IMAGE) --format '  {{.Repository}}:{{.Tag}}  {{.Size}}'

.PHONY: deploy
deploy: docker ## Transférer l'image sur le NAS et redémarrer la stack
	@echo "Transfert de l'image vers $(NAS_HOST)…"
	docker save $(IMAGE) | gzip | ssh $(NAS_HOST) 'gunzip | sudo docker load'
	ssh $(NAS_HOST) 'cd $(NAS_DIR) && sudo docker-compose up -d'
	@echo "Déployé. Logs : ssh $(NAS_HOST) 'cd $(NAS_DIR) && sudo docker-compose logs -f'"

.PHONY: clean
clean: ## Supprimer les artefacts de build
	rm -rf domotic $(FRONTEND)/dist $(API_DIR)/*.json
	# .gitkeep doit survivre : sans lui, le //go:embed du package web ne compile plus.
	find $(BACKEND)/web/dist -mindepth 1 ! -name .gitkeep -delete

# Image du service domotic : frontend Angular compilé puis embarqué dans un
# binaire Go statique. Le contexte de build attendu est la racine du dépôt.
#
#   docker build -f backend/Dockerfile -t domotic:latest .

# --- Étape 1 : build du frontend ------------------------------------------
FROM node:22-alpine AS frontend

WORKDIR /app

# Copier d'abord les manifestes : tant qu'ils ne changent pas, Docker réutilise
# le cache de npm ci, de loin l'étape la plus longue.
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

# --- Étape 2 : build du backend -------------------------------------------
FROM golang:1.27-alpine AS backend

WORKDIR /src

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./

# Le frontend compilé doit être en place avant la compilation : go:embed lit le
# répertoire au moment du build.
COPY --from=frontend /app/dist/frontend/browser ./web/dist

ARG VERSION=dev

# CGO_ENABLED=0 est indispensable : c'est ce qui rend le binaire statique et
# donc exécutable sur distroless/static. Possible uniquement parce que le
# driver SQLite retenu (modernc.org/sqlite) est en Go pur.
#
# GOAMD64 reste à sa valeur par défaut (v1) : le Celeron N3050 du DS216+ ne
# dispose pas d'AVX2, un binaire compilé en v3 refuserait de démarrer.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /domotic ./cmd/domotic

# --- Étape 3 : image finale -----------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot

# distroless/static fournit les certificats CA racine, nécessaires pour joindre
# l'API cloud Netatmo en HTTPS. La CA Overkiz de la box TaHoma, elle, est
# embarquée dans le binaire.
COPY --from=backend /domotic /domotic

USER nonroot:nonroot
EXPOSE 8080

ENV DOMOTIC_DB_PATH=/data/domotic.db

ENTRYPOINT ["/domotic"]

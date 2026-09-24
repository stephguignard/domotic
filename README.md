# Domotic

Service d'agrégation domotique : un binaire Go qui consolide les équipements
**Netatmo** (météo, sécurité), **Somfy TaHoma** (volets, portails) et **Shelly**
(relais) derrière une API REST unifiée, avec une interface Angular embarquée.

Conçu pour tourner sur un **Synology DS216+** — Celeron N3050, 1 Go de RAM —
où Home Assistant ne tient pas. L'image Docker fait moins de 40 Mo et le
service se contente d'une cinquantaine de mégaoctets au repos.

## Architecture

```
                    ┌──────────────────────────────┐
   Netatmo   ──────▶│                              │
   (cloud, OAuth2)  │   domotic (Go, 1 conteneur)  │◀──── navigateur
                    │                              │
   TaHoma    ──────▶│   API REST + SQLite + SPA    │
   (LAN, port 8443) │                              │
   Shelly    ──────▶│                              │
   (LAN, HTTP)      │                              │
                    └──────────────────────────────┘
```

Le frontend ne parle jamais aux API amont : le backend les interroge en tâche
de fond, consolide l'état en base, et sert une vue unifiée. L'interface reste
donc réactive même quand une source est indisponible, et les quotas d'API ne
dépendent pas du nombre d'onglets ouverts.

**Choix structurants :**

- **Un seul conteneur.** Le binaire Go embarque le frontend Angular compilé via
  `embed.FS`. Sur 1 Go de RAM, un nginx séparé serait un coût sans contrepartie.
- **SQLite en Go pur** (`modernc.org/sqlite`), ce qui permet `CGO_ENABLED=0` et
  donc une image `distroless/static`.
- **OpenAPI dérivé du code.** [Huma](https://huma.rocks) produit la spécification
  à partir des types Go ; le client TypeScript en est généré. Le contrat ne peut
  pas diverger de l'implémentation.

## Prérequis

| Outil | Version | Rôle |
|---|---|---|
| Go | 1.25+ | backend |
| Node | 22+ | frontend |
| npm | 12+ | npm 10 échoue sur les peer deps d'Angular 22 |
| Java | 17+ | `openapi-generator-cli` |
| Docker | — | build de l'image de déploiement |

## Démarrage

```bash
cp .env.example .env     # renseigner les identifiants (facultatif au départ)
make dev                 # backend sur :8080, frontend sur :4200
```

Le service démarre sans aucun identifiant : les intégrations non configurées
sont simplement inactives, ce qui permet de les ajouter une par une.

| URL | Contenu |
|---|---|
| http://localhost:4200 | interface (proxie `/api` vers le backend) |
| http://localhost:8080/docs | documentation interactive de l'API |
| http://localhost:8080/api/health | état du service et de ses sources |

Travailler sur `:4200` plutôt que sur `:8080` : le dev-server Angular apporte le
rechargement à chaud et relaie `/api`, `/auth` et `/docs` vers le backend.

### Chaque partie séparément

`make dev` lance les deux processus en parallèle et les arrête ensemble. Pour
isoler les logs d'un côté, deux terminaux :

```bash
make dev-backend     # backend seul sur :8080, logs en mode debug
make dev-frontend    # frontend seul sur :4200
```

Les équivalents directs, quand il faut passer des options :

```bash
cd backend  && go run ./cmd/domotic --debug    # --help pour les options
cd frontend && npx ng serve --port 4300
```

> `go` n'est pas dans le `PATH` par défaut — il est installé dans `~/.local/go` :
> `export PATH=$HOME/.local/go/bin:$PATH`. Le `Makefile` s'en charge, pas un `cd`
> manuel.

### Tester la version compilée

```bash
make build     # compile le frontend et l'embarque dans le binaire
./domotic      # tout sur :8080, un seul processus
```

C'est la forme déployée sur le NAS : le binaire sert lui-même l'interface, il n'y
a plus de `:4200`. La configuration passe alors par l'environnement :

```bash
DOMOTIC_PORT=9000 DOMOTIC_DB_PATH=./data/test.db ./domotic
```

## Configuration des intégrations

### Netatmo

1. Créer une application sur [dev.netatmo.com](https://dev.netatmo.com/apps/).
2. Y déclarer l'URL de redirection `<DOMOTIC_PUBLIC_URL>/auth/netatmo/callback`,
   au caractère près.
3. Renseigner `NETATMO_CLIENT_ID` et `NETATMO_CLIENT_SECRET`.
4. Ouvrir `/auth/netatmo` dans un navigateur et accepter le consentement.

Cette dernière étape n'est à faire qu'une fois. **Netatmo fait tourner le
refresh token à chaque rafraîchissement** et invalide le précédent : le service
persiste donc chaque rotation en base, ce qui lui permet de survivre aux
redémarrages sans ré-authentification. C'est la raison pour laquelle le volume
`/data` doit être sauvegardé.

### Somfy TaHoma

L'API locale évite le cloud Somfy et répond en quelques millisecondes.

1. Dans l'application TaHoma, ouvrir les paramètres de la passerelle et **taper
   sept fois sur son PIN** pour activer le mode développeur.
2. Générer un jeton. Il n'est affiché qu'à sa création — le noter aussitôt.
3. Réserver un bail DHCP fixe à la box, puis renseigner `TAHOMA_HOST` avec son
   **adresse IP**, `TAHOMA_PIN` et `TAHOMA_TOKEN`.

Les trois variables doivent être renseignées **ensemble** : une configuration
partielle fait volontairement échouer le démarrage.

Le PIN se lit dans le certificat que la box présente, sans ouvrir l'application :

```bash
openssl s_client -connect <ip-de-la-box>:443 </dev/null 2>/dev/null \
  | openssl x509 -noout -subject
# subject=O = Overkiz, OU = Overkiz Device Server, CN = 1234-5678-9012.local
```

La même commande vérifie la chaîne TLS contre la CA embarquée, ce qui permet de
valider la connexion avant même d'avoir un jeton :

```bash
openssl s_client -connect <ip>:443 -servername gateway-<pin>.local \
  -CAfile backend/internal/tahoma/overkiz-root-ca-2048.crt \
  -verify_hostname gateway-<pin>.local </dev/null 2>&1 | grep 'Verify return code'
```

> **Le port 8443 fermé alors que le 443 répond** signifie que le mode
> développeur n'est pas activé. Il peut aussi se refermer si le serveur NTP
> distribué par le DHCP est invalide.

L'IP plutôt que `gateway-<pin>.local` : la résolution mDNS depuis un conteneur
Docker sur Synology est peu fiable. Le certificat de la box étant émis pour son
nom `.local`, le client se connecte à l'IP tout en validant ce nom, avec la CA
Overkiz embarquée dans le binaire — sans jamais désactiver la vérification TLS.

### Shelly

Modules **Gen2 et suivants** (Plus, Pro, Gen3, Gen4) ; les Gen1 parlent une
autre API et ne sont pas pris en charge.

1. Réserver un bail DHCP fixe à chaque module, et renseigner leurs **adresses
   IP** dans `SHELLY_HOSTS`, séparées par des virgules.
2. Si l'authentification est activée sur les modules (recommandé), renseigner
   `SHELLY_PASSWORD` — le même pour tous.

Chaque voie d'un module (`switch:0`, `switch:1`…) devient un équipement de type
**relais**, nommé d'après le nom donné à la voie dans l'interface du module.
L'API locale ne connaît pas de pièces. Toute commande sur un relais demande une
confirmation dans l'interface : un chauffe-eau ou un chauffage coupé par erreur
ne se voit pas.

Côté module, penser à **désactiver le point d'accès Wi-Fi** (`Wi-Fi > Access
Point`) : ouvert par défaut, il permet à quiconque à portée de piloter les
relais.

## Commandes

`make help` liste toutes les cibles.

| Commande | Effet |
|---|---|
| `make dev` | backend + frontend en parallèle |
| `make dev-backend` · `make dev-frontend` | une seule des deux parties |
| `make build` | frontend compilé puis embarqué dans le binaire |
| `make build-frontend` · `make build-backend` | une seule étape de build |
| `make openapi` | régénère `api/openapi.json` et `api/openapi-3.0.json` |
| `make api-client` | `openapi`, puis régénère le client TypeScript |
| `make test` | tests Go et Angular |
| `make test-backend` · `make test-frontend` | une seule suite |
| `make lint` | `go vet` + vérification du formatage |
| `make fmt` | formate le code Go |
| `make docker` | image Docker |
| `make deploy` | transfert vers le NAS et redémarrage de la stack |
| `make clean` | supprime les artefacts de build |

Un test isolé :

```bash
cd backend  && go test ./internal/store/ -run TestTokenRoundTrip -v
cd frontend && npx ng test --watch=false --filter "^DeviceList"
```

Les tests Angular tournent sous Vitest dans jsdom : `--filter` prend une
expression régulière testée contre les noms de suites et de tests.

### Faire évoluer l'API

Le contrat part du code Go et descend jusqu'au frontend :

```
internal/api/*.go  →  make api-client  →  frontend/src/app/api/
```

Ajouter un champ à un type Go et lancer `make api-client` suffit à le voir
apparaître, typé, côté Angular. Le répertoire `frontend/src/app/api/` est
git-ignoré : il est entièrement dérivé et ne doit jamais être édité.

> La spécification est produite en OpenAPI 3.1 **et** en 3.0.3. C'est cette
> dernière qui alimente le générateur `typescript-angular`, dont le support de
> la 3.1 reste partiel.

## Déploiement sur le NAS

DSM 7.1 n'a pas Container Manager : les commandes passent par SSH. Sans registre
privé, l'image se transfère directement — le poste de développement et le NAS
partagent l'architecture amd64, aucune cross-compilation n'est nécessaire.

```bash
make deploy NAS_HOST=nas NAS_DIR=/volume1/docker/domotic
```

Préparation du NAS, une seule fois :

```bash
ssh nas 'mkdir -p /volume1/docker/domotic/data'
# L'image distroless tourne en nonroot (UID 65532) : sans ce chown, le service
# échoue au démarrage sur « unable to open database file ».
ssh nas 'sudo chown -R 65532:65532 /volume1/docker/domotic/data'
```

Y déposer également `.env` et `docker-compose.yml`.

Mesuré sur ce squelette : **5,4 Mo de RAM** au repos, pour une image de 17,5 Mo.

Pour essayer l'image localement avant de déployer :

```bash
make docker
mkdir -p ./data && sudo chown -R 65532:65532 ./data
docker run --rm -p 8080:8080 -v $(pwd)/data:/data --memory=128m domotic:latest
```

> Le binaire est compilé avec `GOAMD64=v1`, la valeur par défaut : le Celeron
> N3050 est un Braswell sans AVX2, un binaire en `v3` refuserait de démarrer.

## Structure

```
backend/
  cmd/domotic/         point d'entrée : serve (défaut) et openapi
  internal/
    config/            configuration par variables d'environnement
    store/             SQLite, migrations, persistance des jetons
    api/               opérations Huma et flux OAuth2
    netatmo/           client cloud, rotation du refresh token
    tahoma/            client local, CA Overkiz, flux d'événements
    shelly/            client JSON-RPC Gen2+, authentification digest
    command/           contrat commun des sources pilotables
    poller/            boucles de rafraîchissement
  web/                 frontend embarqué
frontend/
  src/app/
    api/               client généré (git-ignoré)
    core/              état partagé et mise en forme
    features/          dashboard, liste, détail
api/                   spécifications OpenAPI générées
```

## Extensions envisagées

- **Somfy RTS** (volets sans retour d'état) : ESP32 + CC1101 exposant du MQTT,
  et le service `mosquitto` commenté dans `docker-compose.yml`.
- **Webhooks Netatmo** pour la sécurité, qui exigent une URL HTTPS publique —
  d'où le service `cloudflared`, lui aussi prêt à décommenter.

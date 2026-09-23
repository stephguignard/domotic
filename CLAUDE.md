# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Contexte

Service d'agrégation domotique : un backend Go consolide les équipements **Netatmo**
(cloud, OAuth2) et **Somfy TaHoma** (API locale sur le LAN) derrière une API REST
unifiée, et sert lui-même un frontend Angular embarqué.

La cible de déploiement est un **Synology DS216+** : Celeron N3050, **1 Go de RAM**,
DSM 7.1. Cette contrainte explique la plupart des choix d'architecture et doit être
prise en compte avant d'ajouter une dépendance ou un service.

## Environnement

Go n'est pas dans le `PATH` par défaut — il est installé dans `~/.local/go` :

```bash
export PATH=$HOME/.local/go/bin:$PATH
```

Le `Makefile` s'en charge déjà. Dans un appel `Bash` direct, il faut l'exporter.

**npm 12+ est requis.** npm 10 échoue sur les peer deps d'Angular 22
(`Cannot read properties of null (reading 'edgesOut')`). npm 12 bloque aussi les
scripts d'installation : `esbuild` en a besoin et est approuvé via le champ
`allowScripts` de `frontend/package.json`. Après l'ajout d'une dépendance à script
d'installation, `npm install-scripts approve <pkg>`.

## Commandes

```bash
make dev          # backend :8080 + frontend :4200 (proxie /api vers le backend)
make build        # frontend compilé puis embarqué dans le binaire ./domotic
make openapi      # régénère api/openapi.json et api/openapi-3.0.json
make api-client   # openapi puis régénère frontend/src/app/api/
make test         # go test ./... + vitest
make lint         # go vet + vérification gofmt
make docker       # image Docker (~17,5 Mo)
make deploy       # docker save | ssh | docker-compose up
```

Un test isolé :

```bash
cd backend && go test ./internal/store/ -run TestTokenRoundTrip -v
cd frontend && npx ng test --watch=false --filter "^parseState"
```

Angular 22 utilise **Vitest**, pas Karma : `--browsers=ChromeHeadless` n'existe plus,
et `--filter` prend une expression régulière testée contre les noms de suites et de
tests.

Les tests tournent dans **jsdom**, qui n'implémente pas Canvas. `src/test-setup.ts`
stube `getContext()` : sans lui, chaque rendu du composant de détail pollue la sortie
et Chart.js échoue silencieusement. Les specs vérifient donc les **données** du
graphique (`chartData()`), jamais son rendu — cela demanderait le mode navigateur,
via `@vitest/browser-playwright`.

`src/app/testing/providers.ts` regroupe les providers communs et des fabriques
d'équipements. Les specs de composants utilisent le **vrai** `DevicesStore` et
interceptent les requêtes HTTP plutôt que de simuler le store : ce qui est vérifié
est alors le comportement réel, du décodage de la réponse jusqu'au rendu.

## Architecture

### Le frontend ne parle jamais aux API amont

```
Netatmo (cloud) ──┐
                  ├──▶ poller ──▶ SQLite ──▶ API REST ──▶ Angular
TaHoma (LAN)    ──┘
```

`internal/poller` interroge les sources en tâche de fond et consolide l'état en base ;
les handlers ne lisent que SQLite. L'interface reste donc réactive quand une source
est indisponible, et les quotas d'API ne dépendent pas du nombre d'onglets ouverts.

**Conséquence pratique :** un endpoint qui aurait besoin d'une donnée fraîche d'une
source amont doit passer par le poller et la base, pas appeler le client directement.
La seule exception est `POST /api/devices/{id}/command`, qui traverse vers la box
TaHoma parce qu'une commande n'a de sens qu'immédiate.

### Le contrat d'API descend du code Go

```
internal/api/*.go  →  make api-client  →  frontend/src/app/api/
```

Huma dérive l'OpenAPI des types Go annotés. `frontend/src/app/api/` est **git-ignoré
et entièrement généré** : ne jamais l'éditer. Après un changement de type ou de
handler, lancer `make api-client`.

Deux spécifications sont produites : la **3.1** (`api/openapi.json`, servie par Huma)
et la **3.0.3** (`api/openapi-3.0.json`). C'est la 3.0.3 qui alimente le générateur
`typescript-angular`, dont le support de la 3.1 reste partiel.

Deux contraintes portent sur les types exposés :

- **Tags d'opération en ASCII.** Un tag « Équipements » produit un fichier
  `quipements.service.ts` — le générateur écrase les caractères accentués. Les
  `Summary` et `Description` restent en français, seuls les `Tags` sont contraints.
- **Slices de sortie annotés `nullable:"false"`.** Un slice Go nil sérialise en
  `null`, et le client TypeScript serait alors typé `Device[] | null`. Les handlers
  retournent déjà des slices non-nil : l'annotation aligne le contrat sur cette
  garantie.

### Un seul conteneur

`backend/web/embed.go` embarque le build Angular via `//go:embed all:dist` et le
sert, avec un repli SPA sur `index.html` pour toute route inconnue. Sur 1 Go de RAM,
un nginx séparé serait un coût sans contrepartie.

`backend/web/dist/` ne contient que `.gitkeep` dans Git — le répertoire doit exister
pour que la directive `embed` compile. Si le frontend n'a pas été compilé, le handler
sert une page explicative (`placeholderHTML`) et le backend reste pleinement
utilisable. **Toute manipulation de ce répertoire doit préserver `.gitkeep`** ; c'est
ce que font `build-frontend` et `clean` dans le `Makefile`.

## Styles

Le frontend n'utilise plus de SCSS : les styles sont des classes Tailwind 4 écrites
dans les templates. Ne subsistent en CSS que `:host` et les composants Optimus visés
par `styleClass` — qui accepte directement des classes Tailwind, d'où l'absence de
`::ng-deep`.

**L'ordre des couches CSS doit rester cohérent** entre le `@layer theme, base,
optimus, components, utilities` de `src/styles.css` et l'option `cssLayer` passée à
`provideOptimus`. Le rompre laisse le preflight de Tailwind écraser les composants.

Une couleur du thème absente du plugin (qui ne mappe que `primary-*` et `surface-*`)
se déclare dans le `@theme inline` de `src/styles.css` plutôt qu'en valeur arbitraire
dans les templates.

## Pièges des intégrations

### Netatmo : la rotation du refresh token

Chaque appel à `/oauth2/token` renvoie un **nouveau** refresh token et invalide le
précédent. Un `oauth2.ReuseTokenSource` standard garde la rotation en mémoire
seulement : au redémarrage, le jeton relu en base est déjà périmé et l'accès est
perdu jusqu'à une ré-authentification manuelle.

`internal/netatmo/tokensource.go` persiste donc chaque rotation, et **retourne une
erreur** si l'écriture échoue — continuer reviendrait à perdre l'accès silencieusement.
Ne pas transformer cette erreur en simple log.

Le test qui valide réellement ce mécanisme : s'authentifier, **redémarrer le binaire**,
puis vérifier que l'appel suivant passe. Le volume `/data` porte ce jeton.

### TaHoma : IP de connexion, nom de certificat

La box présente un certificat émis pour `gateway-<pin>.local`, signé par une CA
absente des magasins système. On s'y connecte pourtant par **adresse IP** : mDNS est
peu fiable depuis un conteneur Docker sur Synology.

`internal/tahoma/client.go` force donc la destination réseau via `DialContext` tout
en validant `ServerName: gateway-<pin>.local`, avec `overkiz-root-ca-2048.crt`
embarqué. **Ne jamais recourir à `InsecureSkipVerify`** : la CA est publique et
vérifiable, la désactiver n'apporterait rien.

La box expose aussi ses propres composants et ses ponts de protocole comme des
équipements. `isInfrastructure()` les écarte sur le **`controllableName`**, jamais sur
le préfixe de `deviceURL` : filtrer `zigbee://` écarterait un vrai équipement Zigbee
appairé plus tard, alors que seul le coordinateur porte un `Transceiver`. Seule la
migration `0002` s'appuie sur le préfixe, faute de `controllableName` stocké.

Le flux d'événements (`internal/tahoma/events.go`) tolère un listener expiré — la box
les recycle — en le réenregistrant et en retentant une fois. La box impose **un appel
par seconde maximum** sur `/events/{id}/fetch` ; `config.validate()` refuse un
intervalle plus court.

## Contraintes de déploiement

- **`CGO_ENABLED=0` est obligatoire.** C'est ce qui produit un binaire statique
  compatible `distroless/static`, et c'est possible uniquement parce que le driver
  SQLite (`modernc.org/sqlite`) est en Go pur. Ne pas le remplacer par
  `mattn/go-sqlite3`, qui exige CGO.
- **Ne pas fixer `GOAMD64=v3`.** Le Celeron N3050 est un Braswell sans AVX2 ; le
  binaire refuserait de démarrer.
- **Volume en UID 65532.** L'image distroless tourne en `nonroot` : sans
  `chown -R 65532:65532` sur le répertoire monté, le service échoue au démarrage sur
  `unable to open database file`.

## Configuration

Toute la configuration passe par l'environnement (`internal/config`), validée au
démarrage. **Le Makefile charge `.env`** — docker-compose le fait nativement, `make`
non : sans cela les identifiants resteraient invisibles en développement. Deux
conséquences : `DOMOTIC_DB_PATH` porte dans `.env.example` le chemin de
développement (docker-compose impose le sien), et **tout test de configuration doit
appeler `isolate(t)`**, sans quoi une configuration réelle le fait échouer.

Le service démarre **sans aucun identifiant** : une intégration non configurée reste
inerte, ce qui permet de les activer une par une.

En revanche, une configuration **partiellement** renseignée fait échouer le démarrage —
c'est presque toujours une faute de frappe, et une intégration silencieusement
inactive coûte plus cher à diagnostiquer qu'un refus net.

## Mémoire de projet

Le contexte long vit dans `.memory-bank/domotic/` (memory-bank façon Cline) :
`projectbrief`, `productContext`, `systemPatterns`, `techContext`, `activeContext`,
`progress`, plus `journal.md` (journal daté, entrée la plus récente en haut, jamais
réécrit) et `decisions.md` (décisions façon ADR). Les notes d'agent denses vivent
dans `.serena/memories/`, avec `core` pour point d'entrée.

`/hello` charge cette mémoire en début de session ; `/bye` met à jour
`activeContext` / `progress`, ajoute une entrée au journal et consigne les choix
structurants dans `decisions.md`. Ce fichier reste la référence des règles : la
mémoire ne doit pas le contredire.

Les serveurs MCP `serena` et `memory-bank` sont déclarés dans `.mcp.json`,
git-ignoré car il contient des chemins propres à la machine.

## Langue

Code, commentaires, messages de log, d'erreur et de commit sont en **français**. Les
identifiants de code, les tags OpenAPI et les noms de champs JSON restent en anglais.

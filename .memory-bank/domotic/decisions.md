# Decisions — domotic

Journal de décisions (façon ADR). Chaque entrée : **Contexte → Décision →
Conséquences**, plus un statut. Plus récente en bas ; ne jamais réécrire — remplacer
par une nouvelle entrée.

Statuts : `accepted` · `superseded by #N` · `proposed`

---

## 1. Un seul binaire, un seul conteneur
_Statut : accepted (fondateur)_

**Contexte.** NAS à 1 Go de RAM, Home Assistant trop lourd.
**Décision.** Binaire Go qui embarque le build Angular (`embed.FS`) ; pas de nginx.
**Conséquences.** `backend/web/dist/.gitkeep` doit être préservé ; image ~17,5 Mo.

---

## 2. SQLite en Go pur, `CGO_ENABLED=0`
_Statut : accepted_

**Contexte.** Image `distroless/static`, binaire statique.
**Décision.** `modernc.org/sqlite`, jamais `mattn/go-sqlite3`.
**Conséquences.** Build sans toolchain C ; migrations goose embarquées.

---

## 3. Le frontend ne parle jamais aux API amont
_Statut : accepted_

**Contexte.** Sources parfois indisponibles, quotas d'API.
**Décision.** Poller → SQLite → API. Seule exception : `POST /api/devices/{id}/command`.
**Conséquences.** UI réactive hors ligne ; une donnée fraîche passe par le poller.

---

## 4. Contrat d'API dérivé du code Go (Huma)
_Statut : accepted_

**Décision.** OpenAPI 3.1 + 3.0.3 générées depuis les types Go ; client
`typescript-angular` généré depuis la 3.0.3, git-ignoré.
**Conséquences.** Tags ASCII, slices `nullable:"false"`, `make api-client` après
tout changement de type.

---

## 5. TaHoma par IP avec validation du nom de certificat
_Statut : accepted_

**Contexte.** mDNS peu fiable depuis Docker sur Synology.
**Décision.** Dial par IP, `ServerName: gateway-<pin>.local`, CA Overkiz embarquée.
**Conséquences.** Jamais `InsecureSkipVerify`.

---

## 6. Persistance de chaque rotation du refresh token Netatmo
_Statut : accepted_

**Décision.** Token source maison qui écrit chaque rotation en base et échoue si
l'écriture échoue.
**Conséquences.** Volume `/data` à sauvegarder.

---

## 7. Écarter les passerelles internes TaHoma
_Statut : superseded by #10_

**Décision.** Filtrer sur le préfixe de `deviceURL` (`ogp://`, `internal://`,
`zigbee://`), purge via migration `0002`.

---

## 8. Styles en Tailwind 4 plutôt qu'en SCSS
_Statut : accepted_

**Décision.** CSS + Tailwind, styles écrits en classes utilitaires dans les templates.

---

## 9. Serena + memory-bank pour la mémoire de projet
_Statut : accepted_

**Décision.** Mémoire longue dans `.memory-bank/domotic/` (versionnée), notes
d'agent denses dans `.serena/memories/`, `CLAUDE.md` reste la référence des règles.
**Conséquences.** `/hello` en début de session, `/bye` en fin.

---

## 10. Filtrer l'infrastructure TaHoma sur le `controllableName`
_Statut : accepted — remplace #7_

**Contexte.** #7 décrivait le critère comme portant sur le préfixe de `deviceURL` ;
c'est inexact, et ce critère serait faux à terme.
**Décision.** `isInfrastructure()` teste le `controllableName` : préfixe `internal:`,
suffixe `:bridge`, ou contenant `transceiver`. Le préfixe de protocole n'est utilisé
que par la migration `0002`, faute de `controllableName` stocké en base.
**Conséquences.** Un équipement Zigbee appairé plus tard est conservé — seul le
coordinateur porte un `Transceiver`, alors que filtrer `zigbee://` l'aurait écarté.
Un équipement légitime effacé par la migration est réinséré au polling suivant.

---

## 11. `.env` chargé par le Makefile
_Statut : accepted_

**Contexte.** Le README demandait de créer `.env` avant `make dev`, mais seul
docker-compose le lisait : les identifiants restaient invisibles en développement,
sans que rien ne le signale.
**Décision.** Le Makefile inclut `.env` et n'exporte que les clés qui y figurent.
**Conséquences.** `DOMOTIC_DB_PATH` dans `.env.example` doit porter le chemin de
développement — docker-compose impose le sien. Tout test de configuration doit
appeler `isolate(t)` : deux tests dépendaient de l'environnement de la machine et
ne passaient que par chance.

---

## 12. Couleurs du thème Optimus déclarées dans `@theme`
_Statut : accepted — complète #8_

**Contexte.** Le plugin Tailwind d'Optimus ne mappe que `primary-*` et `surface-*` ;
le reste du thème vit dans des variables `--p-*` hors de portée des utilitaires.
**Décision.** Déclarer les couleurs et rayons manquants dans le `@theme inline` de
`src/styles.css` (`text-muted`, `border-divider`, `bg-highlight`, `rounded-content`…).
**Conséquences.** Pas de `text-[var(--p-…)]` semé dans les templates ; `inline`
préserve le basculement clair/sombre. L'ordre `theme, base, optimus, components,
utilities` et l'option `cssLayer` de `provideOptimus` doivent rester cohérents,
sinon le preflight de Tailwind écrase les composants.

---

## 13. Travail en branches et pull requests
_Statut : accepted_

**Contexte.** Les douze premiers commits ont été poussés directement sur `main`.
`gh` a été installé pour ouvrir des PR.
**Décision.** Toute modification part d'une branche `<type>/<sujet>` et revient par
une pull request. `gh` vit dans `~/.local/bin` : `sudo` réclame un mot de passe sur
cette machine, le paquet système n'était pas une option.
**Conséquences.** `CLAUDE.md` gagne une section « Travail en branches ». La règle y
prescrit un fast-forward tant que `main` n'a pas divergé, afin de garder l'historique
linéaire — **mais la PR #1 a été fusionnée avec un merge commit**, ce qui l'a
contredite dès son premier usage. Stratégie à trancher : ajuster la règle, ou régler
GitHub pour n'autoriser que le fast-forward.

---

## 14. Deux conventions de commit coexistent
_Statut : superseded by #15_

**Contexte.** `.claude/rules/conventional-commits.md` a été versionné tel quel, à la
demande. Il impose Conventional Commits : `feat(scope): add …`, en anglais, à
l'impératif. `CLAUDE.md` et `.serena/memories/conventions.md` imposent l'inverse :
une phrase française au présent, sans préfixe — ce que suivent les douze premiers
commits, le treizième (`docs: add the branch-and-PR workflow…`) étant le premier
écart.

Le fichier importé porte des traces de son projet d'origine (`angular-state-example`,
cité par le journal comme modèle d'organisation) : ses scopes sont `todo`, `user`,
`invoice`, `dynform`, `cva`, `formly`, et il attribue les commits à Sonnet 5.

**Décision.** Aucune pour l'instant : les deux règles coexistent, la contradiction est
documentée ici et dans la PR #1 plutôt que résolue à la hâte.
**Conséquences.** Trois issues possibles — adopter Conventional Commits en adaptant
les scopes au projet (`tahoma`, `netatmo`, `store`, `api`, `frontend`, `memory-bank`),
revenir au français sans préfixe et retirer le fichier, ou un compromis (type préfixé,
description française). Tant que rien n'est tranché, préciser la convention voulue
avant de demander un commit.

---

## 15. Conventional Commits pour les nouveaux messages
_Statut : accepted — tranche #14_

**Contexte.** #14 laissait coexister deux conventions. L'arbitrage a été rendu.
**Décision.** Les nouveaux messages suivent `.claude/rules/conventional-commits.md` :
`<type>(<scope>): <description>`, en anglais, à l'impératif présent. Le fichier de
règles reste inchangé, scopes hérités compris.
**Conséquences.** `CLAUDE.md` ne réclame plus le français pour les messages de commit
— le reste (code, commentaires, logs, erreurs) y demeure en français. **L'historique
antérieur n'est pas réécrit** : les douze premiers commits gardent leurs sujets
français sans préfixe, et la rupture se lit à partir de `81b9927`. Les scopes du
fichier appartiennent encore à `angular-state-example` ; employer ceux du projet
(`tahoma`, `netatmo`, `store`, `api`, `frontend`, `memory-bank`, `deps`, `tooling`)
ou omettre le scope si le changement est transverse.


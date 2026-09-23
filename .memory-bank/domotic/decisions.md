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
_Statut : accepted_

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

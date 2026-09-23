# Journal — domotic

Journal de travail daté, **entrée la plus récente en haut**. Ne jamais réécrire une
entrée passée.

Modèle :

```
## AAAA-MM-JJ — titre court
Done: …
Decided: …
Observed: …
Next: …
```

---

## 2026-09-23 — Flux branches + pull requests
Done: `gh` 2.101.0 installé dans `~/.local/bin` et authentifié (device flow, compte
`stephguignard`). Section « Travail en branches » ajoutée à `CLAUDE.md` et à la
mémoire Serena. `.claude/rules/conventional-commits.md` versionné tel quel. PR #1
créée puis fusionnée — première du dépôt. Branches `chore/memoire-projet` et
`docs/convention` supprimées en local et sur GitHub.
Decided: plus de commit direct sur `main` ; une branche `<type>/<sujet>` puis une PR.
Le fichier de règles reste inchangé sur demande, malgré ses scopes hérités d'un autre
projet.
Observed: le fichier de règles **contredit** `CLAUDE.md` et Serena sur la convention
de commit (anglais préfixé vs français sans préfixe) — contradiction consignée dans la
PR #1 plutôt que masquée. La PR a été fusionnée avec un merge commit alors que
`CLAUDE.md` prescrit le fast-forward : premier écart à la règle qu'elle venait de
poser. `gh auth login` lancé sans entrée standard prend les valeurs par défaut, d'où
un protocole `https` retenu là où le remote est en SSH.
Next: trancher la convention de commit et la stratégie de fusion ; connecter Netatmo ;
`make docker` avant tout déploiement.

## 2026-09-23 — Intégration TaHoma validée sur matériel réel
Done: box branchée (IP fixe, mode développeur), quatre volets remontés, commande
`close` exécutée — le volet du salon s'est physiquement fermé. Passerelles internes
écartées sur le `controllableName` + migration `0002`. `.env` chargé par le Makefile,
tests de config isolés en conséquence. Routes `/auth/netatmo` répondant 503 explicite
sans identifiants. SCSS → CSS + Tailwind 4 puis styles réécrits en utilitaires
(400 lignes de CSS → 92). Docker éprouvé : 17,5 Mo d'image, 6,8 Mo de RAM, accès LAN
à la box depuis le conteneur.
Decided: filtrer sur le `controllableName` et non sur le préfixe de `deviceURL` ;
déclarer les couleurs du thème Optimus dans `@theme` plutôt que de semer des valeurs
arbitraires ; faire lire `.env` par `make`.
Observed: le PIN se lit dans le certificat servi sur :443 — :8443 fermé signifie mode
développeur inactif. `/setup` local ne porte ni `rootPlace` ni `placeOID`, et `/history`
n'existe pas (suivre une commande via `/exec/current`). Charger `.env` a cassé deux
tests de config qui ne passaient que par chance, et a d'abord fait échouer `make dev`
sur le `DOMOTIC_DB_PATH` du conteneur. `ng serve` survit à l'arrêt de `make dev` par
signal.
Next: connecter Netatmo (client_id/secret manquants) puis éprouver la rotation du
refresh token par un redémarrage ; trancher le sort des pièces TaHoma ;
`make docker` avant tout déploiement ; éprouver `make deploy` sur le NAS.

## 2026-09-23 — Mise en place de Serena et du memory-bank
Done: config Serena (`.serena/project.yml`, LSP go + typescript), `.mcp.json`
(serveurs `serena` et `memory-bank`, git-ignoré), memory-bank initial, mémoires
Serena, commandes `/hello` et `/bye`, section « Memory bank » dans `CLAUDE.md`.
Decided: même organisation que `angular-state-example`, contenu en français.
Observed: historique de 11 commits, du squelette (`d91e81c`) au passage à Tailwind (`526e3fe`).
Next: relancer `make test` / `make lint` après Tailwind ; connecter Netatmo.

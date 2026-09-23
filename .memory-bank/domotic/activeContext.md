# Active Context — domotic

_Dernière mise à jour : 2026-09-23 (mise en place du flux branches + PR)_

## Branche courante

`main`, HEAD `104d081`, alignée sur `origin/main`. Arbre propre. **Seule branche du
dépôt** : `chore/memoire-projet` et `docs/convention` ont été fusionnées puis
supprimées, en local comme sur GitHub.

## Récemment fait

- **Flux branches + PR adopté** : plus de commit direct sur `main`. Branche
  `<type>/<sujet>`, puis pull request. Consigné dans `CLAUDE.md` et la mémoire Serena.
- `gh` 2.101.0 installé dans `~/.local/bin` (sudo réclame un mot de passe ici) et
  authentifié sur le compte `stephguignard` — portées `repo`, `read:org`, `gist`.
- **PR #1 créée et fusionnée**, première du dépôt.
- `.claude/rules/conventional-commits.md` versionné **tel quel**, sans adaptation.
- **Conventional Commits adopté** (décision #15) : les nouveaux messages sont en
  anglais préfixés ; l'historique antérieur n'est pas réécrit. Le fichier de règles
  garde les scopes d'`angular-state-example` — employer ceux du projet.

Côté applicatif, rien n'a bougé depuis la clôture précédente : TaHoma validé en réel,
passerelles internes écartées, Tailwind en place, Docker éprouvé.

## Décisions ouvertes

- **Stratégie de fusion** — `CLAUDE.md` prescrit le fast-forward, mais la PR #1 a été
  fusionnée avec un merge commit. Soit la règle change, soit le réglage GitHub.
- Pièces TaHoma : l'API locale n'en fournit aucune → groupement par type, ou mapping
  manuel en configuration ? Rien n'est engagé.
- Retour d'état des volets RTS : récepteur ESP32 + CC1101 exposant du MQTT (idée
  seulement).
- Robustesse de l'arrêt de `make dev` : le `trap` ne tue pas `ng serve` quand l'arrêt
  vient d'un signal plutôt que d'un `Ctrl+C`.

## Attention

- État de volet vide et « Sans pièce » = normal, pas des bugs (RTS + API locale).
- **Netatmo jamais connecté** : il manque `client_id` / `client_secret`. Le test réel
  de la rotation du refresh token (auth → redémarrage → appel suivant) reste à faire ;
  c'est le seul qui valide vraiment le mécanisme.
- L'image Docker construite contient encore le frontend d'avant Tailwind :
  `make docker` avant tout déploiement.
- `make dev` et le conteneur se disputent le port 8080 : n'en lancer qu'un.
- `gh` a retenu **HTTPS** alors que le remote est en SSH ; sans effet sur les PR,
  `gh config set git_protocol ssh` pour aligner.

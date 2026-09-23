---
description: Clôture de session — met à jour le memory-bank, les mémoires Serena et le journal
allowed-tools: Bash(git status:*), Bash(git log:*), Bash(git diff:*), Read, Edit, Write, Glob, Grep, mcp__serena__list_memories, mcp__serena__read_memory, mcp__serena__write_memory, mcp__memory-bank__memory_bank_read, mcp__memory-bank__memory_bank_write, mcp__memory-bank__memory_bank_update
argument-hint: (optionnel) note libre sur la session
---

Objectif : persister l'état de cette session de travail avant de partir. Note
éventuelle de l'utilisateur : $ARGUMENTS

## 1. Fais le point sur la session

- !`git log --oneline main..HEAD`
- !`git status --short`
- !`git diff --stat`
- Relis mentalement ce qui a été fait/décidé/cassé dans cette conversation.

## 2. Mets à jour le memory-bank (`.memory-bank/domotic/`)

Ces fichiers sont **réécrits** pour refléter l'état *actuel* :

- **`activeContext.md`** — « Récemment fait », « Décisions ouvertes », « Attention » :
  remets à jour, enlève ce qui est terminé.
- **`progress.md`** — statut build/tests (relance `make test` / `make lint` si tu as
  changé du code et que le doute existe ; sinon garde le dernier résultat connu et
  date-le), état par composant, problèmes connus.

Ces fichiers sont **append-only** :

- **`journal.md`** — ajoute une entrée **en haut** (juste après le bloc modèle,
  avant l'entrée précédente), datée d'aujourd'hui, au format
  `Done: / Decided: / Observed: / Next:`. Concis, factuel.
- **`decisions.md`** — pour **chaque choix structurant** fait dans la session,
  ajoute une entrée numérotée à la fin (Contexte → Décision → Conséquences +
  statut). Si une décision existante est remise en cause, ne la réécris pas :
  passe-la en `superseded by #N` et crée #N.

## 3. Mets à jour les mémoires Serena

- `mcp__serena__list_memories` puis, pour toute connaissance projet durable
  apprise cette session (piège récurrent, contrainte, commande non triviale),
  `mcp__serena__write_memory` (mets à jour une mémoire existante plutôt que d'en
  créer une quasi-identique ; respecte `memory_maintenance`).
- Ne duplique pas ce que le memory-bank ou `CLAUDE.md` disent déjà.

## 4. Cohérence

- Si une convention ou une commande a changé, répercute-la dans `CLAUDE.md`.
- Vérifie que memory-bank, `CLAUDE.md` et mémoires Serena ne se contredisent pas.

## 5. Rends la main

- Liste chaque fichier touché (une ligne chacun).
- Propose un message de commit (français, présent, sans préfixe — cf. historique)
  et **demande** si tu commites. Si l'utilisateur a déjà dit de committer, fais-le.

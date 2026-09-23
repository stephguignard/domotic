---
description: Charge la mémoire du projet (memory-bank + Serena) et fait un état des lieux
allowed-tools: Bash(git branch:*), Bash(git status:*), Bash(git log:*), Bash(git rev-list:*), Read, Glob, mcp__serena__initial_instructions, mcp__serena__list_memories, mcp__serena__read_memory, mcp__memory-bank__list_projects, mcp__memory-bank__list_project_files, mcp__memory-bank__memory_bank_read
---

Objectif : reprendre le fil du projet `domotic`. Charge la mémoire, puis produis un
**état des lieux court**. Ne modifie rien.

## Contexte git (déjà chargé)

- Branche : !`git branch --show-current`
- Statut : !`git status --short`
- Derniers commits : !`git log --oneline -8`
- Avance sur main : !`git rev-list --count main..HEAD` commit(s)

## Mémoire à charger

1. Memory-bank (fichiers ci-dessous déjà inclus) :

@.memory-bank/domotic/activeContext.md
@.memory-bank/domotic/progress.md

2. Lis en plus, avec Read :
   - `.memory-bank/domotic/journal.md` — **uniquement l'entrée la plus récente**
   - `.memory-bank/domotic/decisions.md` — les décisions au statut `proposed` ou `superseded`
   - `.memory-bank/domotic/projectbrief.md` si tu ne connais pas encore le projet

3. Serena : appelle `mcp__serena__initial_instructions`, puis `mcp__serena__list_memories`
   et lis `core` puis les mémoires pertinentes. Si les serveurs MCP `serena` /
   `memory-bank` ne sont pas disponibles, dis-le et continue avec les fichiers seuls.

## Sortie attendue (format)

Un résumé de ~15 lignes max :

- **Branche & git** : nom, propre/sale, avance sur main
- **En cours / dernier travail** : d'après `activeContext` + dernière entrée `journal`
- **Build / tests** : dernier état connu (d'après `progress`)
- **Décisions ouvertes** : liste courte (statut `proposed` + « Décisions ouvertes » d'`activeContext`)
- **Prochaines étapes suggérées** : 2-4 puces actionnables

Termine par une question : « On reprend sur quoi ? »

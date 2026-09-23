# Progress — domotic

_Dernier état connu : 2026-09-23_

## Build / tests

- `make test` (go test + vitest) : vert au dernier commit de tests (`729084c`) — non
  relancé depuis le passage à Tailwind.
- `make lint` : non relancé cette session.

## Par composant

| Composant | État |
|---|---|
| Config / store / migrations | ✅ fonctionnels, testés |
| TaHoma (client, events, commandes) | ✅ validé en réel sur la box |
| Netatmo (OAuth2, rotation jeton) | 🟡 implémenté, jamais connecté en réel |
| Poller | ✅ |
| API REST + OpenAPI + client généré | ✅ |
| Frontend (dashboard, liste, détail) | ✅ testé (Vitest) |
| Déploiement NAS | 🟡 chaîne prête (`make deploy`), à éprouver |

## Problèmes connus

- Aucune pièce côté TaHoma : tout tombe dans « Sans pièce ».
- Volets RTS sans état.

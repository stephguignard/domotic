# Tests

## Frontend (Vitest + jsdom)
- Pas de Karma. `--filter` regex ; pas de `--browsers`.
- jsdom sans Canvas : `src/test-setup.ts` stube `getContext()`. Vérifier les
  **données** du graphique (`chartData()`), jamais son rendu.
- `src/app/testing/providers.ts` : providers communs + fabriques d'équipements.
- Specs de composants : **vrai** `DevicesStore`, requêtes HTTP interceptées
  (`HttpTestingController`) — ne pas mocker le store.

## Backend
- Tests à côté du code (`*_test.go`) : config, store (round-trip jetons), types
  TaHoma, embed web.
- Test réel de la rotation Netatmo : auth → redémarrer le binaire → appel suivant OK.

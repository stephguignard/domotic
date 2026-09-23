# Conventions

## Langue
- Code commenté, logs, erreurs, commits : **français**.
- Identifiants, tags OpenAPI, champs JSON : anglais.
- Commits : phrase au présent de l'indicatif, 3e personne, sans préfixe
  (« Écarte l'infrastructure de la passerelle TaHoma »).

## Go
- Erreurs enveloppées en français : `fmt.Errorf("configuration de goose: %w", err)`.
- Logs via `log/slog`.
- Nouvelle migration = nouveau fichier `NNNN_*.sql`, jamais modifier une existante.
- Types exposés Huma : `Tags` ASCII, `Summary`/`Description` en français,
  slices de sortie `nullable:"false"` et non-nil.

## Angular
- Standalone, `loadComponent` dans `app.routes.ts`, `inject()`.
- Store : signaux privés, API publique en `asReadonly()` / `computed()`,
  mutations par méthodes nommées.
- Logique de présentation pure dans `core/device-state.ts`.
- Styles : classes Tailwind dans les templates (plus de SCSS).

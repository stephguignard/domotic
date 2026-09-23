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
- Styles : classes Tailwind dans les templates (plus de SCSS). Ne restent en CSS
  que `:host` et les composants Optimus visés par `styleClass`.
- Couleur ou rayon du thème Optimus absent du plugin (`primary-*`, `surface-*`
  seuls) : le déclarer dans le `@theme inline` de `src/styles.css` plutôt que de
  semer `text-[var(--p-…)]` dans les templates. `inline` conserve le basculement
  clair/sombre.
- Ajuster un composant Optimus passe par `styleClass="<classes tailwind>"` ;
  plus aucun `::ng-deep`.
- Classe sans style conservée = sélecteur de test (`room-title`, `reading`,
  `commands`, `device-row`, `row-commands`, `state-row`) : ne pas la supprimer.

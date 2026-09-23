# Conventions

## Langue
- Code commenté, logs, erreurs : **français**.
- Identifiants, tags OpenAPI, champs JSON : anglais.
- Commits : **Conventional Commits en anglais** (`<type>(<scope>): <description>`,
  impératif présent), cf. `.claude/rules/conventional-commits.md`. Les douze premiers
  commits précèdent la règle (sujets français sans préfixe) ; ne pas réécrire.
  Scopes du projet : `tahoma`, `netatmo`, `store`, `api`, `frontend`, `memory-bank`,
  `deps`, `tooling` — ceux du fichier de règles viennent d'un autre projet.

## Git
- Jamais de commit direct sur `main` : branche `<type>/<sujet>` puis PR
  (`gh pr create --fill`). `gh` vit dans `~/.local/bin`.
- Fusion en fast-forward tant que `main` n'a pas divergé ; branche supprimée ensuite.
- Ne jamais committer ni pousser sans demande explicite.
- `gh` authentifié sur `stephguignard`, protocole retenu `https` (remote en SSH ;
  sans effet sur les PR). `gh auth login` sans stdin prend les valeurs par défaut.

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

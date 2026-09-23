# Conventional Commits — règles du dépôt

Tous les messages de commit **doivent** suivre la spécification
[Conventional Commits 1.0.0](https://www.conventionalcommits.org/fr/v1.0.0/).
Ces règles s'appliquent aux commits créés à la main comme par Claude Code.

## Format

```
<type>(<scope>)?(!)?: <description>

<body facultatif>

<footers facultatifs>
```

- **Ligne de sujet** : `<type>(<scope>): <description>`
  - en minuscules, à l'impératif présent (« add », pas « added » / « adds »)
  - pas de point final
  - ≤ 72 caractères si possible
- **Corps** (facultatif) : séparé du sujet par une ligne vide ; explique le *pourquoi*,
  pas le *comment*. Listes à puces `-` acceptées.
- **Footers** (facultatifs) : séparés du corps par une ligne vide. Un par ligne,
  format `Token: valeur` (ou `BREAKING CHANGE: …`).

## Types autorisés

| Type | Usage |
|---|---|
| `feat` | nouvelle fonctionnalité visible par l'utilisateur (nouveau pattern, nouvelle page…) |
| `fix` | correction de bug |
| `docs` | documentation seule (`CLAUDE.md`, `README.md`, `.memory-bank/**`, `.claude/**`) |
| `style` | formatage sans impact sur le sens (espaces, quotes, indentation) |
| `refactor` | changement de code sans nouvelle feature ni fix |
| `perf` | amélioration de performance |
| `test` | ajout / correction de tests uniquement |
| `build` | build, dépendances, config de bundling (`angular.json`, `package.json`, `tsconfig*`) |
| `ci` | configuration d'intégration continue |
| `chore` | tâches de maintenance sans catégorie ci-dessus (config outillage, `.gitignore`…) |
| `revert` | annulation d'un commit précédent |

⚠️ **`feature:` n'est pas un type valide** — utiliser `feat:`. L'historique ancien
en contient (`feature: …`) ; ne pas le reproduire.

## Scopes recommandés

Le scope est **facultatif** mais encouragé dès qu'un commit ne touche qu'une zone.

- **Features** : `todo`, `user`, `invoice`, `dynform`, `cva`, `home`
- **Transverses** : `memory-bank`, `deps`, `config`, `tooling`, `theming`, `formly`
- Un scope large est préférable à un scope trop précis ; omettre le scope si le
  changement est vraiment transverse.

## Breaking changes

- `!` après le type/scope : `feat(invoice)!: …`
- **et/ou** un footer `BREAKING CHANGE: <description de la rupture>`

## Footers d'attribution

Quand Claude Code produit le commit, ajouter après une ligne vide :

```
Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
Claude-Session: <url de session fournie par le harness>
```

(La consigne de session en cours prime sur le nom/modèle exact à utiliser.)

## Exemples

```
feat(cva): add signal-based touched/dirty tracking to AmountCvaComponent
fix(invoice): stop the URL-sync effect from looping on identical query
test(todo): cover the TodoStore SignalStore (computed, loadByQuery debounce)
docs(memory-bank): session wrap-up — Jest migration
build(deps): migrate the unit runner from Karma to Jest
refactor(user): extract the shared query-filter shape into a model
feat(dynform)!: switch repeat-table rows to a required FormArray

BREAKING CHANGE: existing models with an optional rows array no longer validate.
```

## Rappels de portée

- **Ne jamais committer ni pousser sans demande explicite de l'utilisateur.**
- Si sur `main`, créer une branche d'abord (voir consignes harness).
- Un commit = un changement cohérent ; ne pas mélanger `feat` et `refactor` non lié.

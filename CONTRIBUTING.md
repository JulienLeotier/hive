# Contribuer à Hive

Merci de l'intérêt. Hive est petit, local, mono-utilisateur par design —
ce qui ne veut pas dire mal collaboratif. Ce doc explique comment
pousser un changement proprement.

## Workflow

1. **Fork + branche** — `main` est verrouillée, impossible de push
   dessus direct. Crée une branche (`feat/<slug>` ou `fix/<slug>`)
   depuis `main`, pousse, ouvre une PR.
2. **PR template** — rempli quand tu ouvres. Le test plan est un
   minimum, pas une case à cocher automatique.
3. **CI verte obligatoire** — 5 jobs :
   - `Test` (Go 1.26, coverage core ≥ 55%)
   - `Lint (golangci-lint)` — 0 issues
   - `Postgres Integration`
   - `Vulnerabilities` (govulncheck + npm audit)
   - `Frontend (svelte-check)` + vitest
4. **Review** — CODEOWNERS request auto le propriétaire. Pour un solo
   dev, self-approve est OK ; avec des contributeurs la règle passera
   à 1 review minimum.
5. **Squash merge** recommandé (commit histoire propre sur `main`).

## Setup local

```bash
# Go
go build ./...
go test ./...

# Lint (pinned v2.x)
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
golangci-lint run --timeout 5m

# Dashboard
cd web
npm ci
npm run check     # svelte-check (types)
npm test          # vitest
npm run dev       # HMR sur :5173
```

## Conventions

### Commits

Format court impératif, français accepté :
```
feat(bmad): per-AC review parsing
fix(ui): bouton Annuler ne s'affiche que si une skill tourne
refactor: split intake.go en pipeline + observer
test(api): couvre handleResume
docs: README à jour
```

Préfixes habituels : `feat`, `fix`, `refactor`, `test`, `docs`,
`chore`, `ci`, `perf`.

### Code Go

- `gofmt` / `goimports` (le CI vérifie via golangci-lint)
- Pas de commentaires "what" — les identifiants bien nommés suffisent.
  Commentaires uniquement pour "why" non-évident (workaround d'un
  bug, invariant caché, contrainte externe).
- Tests : `internal/<pkg>/<file>_test.go`. La coverage core
  (bmad + devloop + event + intake + project) doit rester ≥ 55%.
- SQL : migrations append-only dans `internal/storage/migrations/`
  (SQLite) + `internal/storage/migrations/postgres/`. Ne jamais
  modifier une migration déjà mergée — rajoute une nouvelle.
- Handlers API : validation stricte des inputs, wrap errors via
  `writeError(w, status, code, msg)`.

### Code TypeScript / Svelte

- `svelte-check` doit passer (0 errors, 0 warnings).
- Svelte 5 runes (`$state`, `$derived`, `$effect`, `$props`) — pas
  de legacy `export let`.
- CSS vars sémantiques (`--text`, `--bg-panel`, `--accent`, …) plutôt
  que hex en dur pour supporter dark/light auto.
- Tous les `window.confirm` / `window.alert` sont interdits — utilise
  `confirmDialog` (`$lib/confirm`).

### Tests

- Backend : `_test.go`, table-driven quand pertinent, un test =
  un comportement observable.
- Frontend : `*.test.ts` avec vitest. Composants testés via
  `@testing-library/svelte`.

## Zones sensibles

- `internal/bmad/` — toute modif peut changer le comportement BMAD
  au runtime. Ajoute un test par skill/format quand c'est possible.
- `internal/storage/migrations/` — append-only. Une migration ratée
  en prod = projet avec data corrompue.
- `internal/api/openapi.yaml` — source de vérité de l'API publique.
  Tout nouveau endpoint doit être documenté ici.

## Rapport de sécurité

Pour une vulnérabilité critique, ne pas ouvrir d'issue publique.
Voir [SECURITY.md](SECURITY.md).

## Licence

En contribuant, tu acceptes que ton code soit publié sous Apache-2.0.

# Hive

**Usine à produits BMAD en local.** Tu décris une idée, Hive lance le
framework [BMAD-METHOD](https://github.com/bmad-code-org/BMAD-METHOD) de
bout en bout via Claude Code — PRD, architecture, stories, implémentation,
revue de code, rétrospective — jusqu'à ce que le produit soit livré.

Un seul binaire Go, un dashboard SvelteKit, une base SQLite. Tout se
pilote depuis le navigateur.

---

## Flow

1. **Nouveau projet** (`/projects`) — tu décris l'idée. Optionnel :
   clone un repo GitHub existant ou pointe un repo local (mode
   brownfield).
2. **Intake PM** — un agent te pose 5 questions (audience, flows,
   non-goals, tech, DoD) pour nourrir le brief BMAD.
3. **Finalisation** → Hive exécute la séquence BMAD officielle, une
   skill par invocation `claude --print` :

   **Greenfield** (13 skills) :
   ```
   /bmad-agent-analyst → /bmad-product-brief
   /bmad-agent-pm → /bmad-create-prd → /bmad-validate-prd
   /bmad-agent-ux-designer → /bmad-create-ux-design
   /bmad-agent-architect → /bmad-create-architecture
   /bmad-agent-pm → /bmad-create-epics-and-stories
   /bmad-agent-architect → /bmad-check-implementation-readiness
   /bmad-agent-dev → /bmad-sprint-planning
   ```

   **Brownfield** (14 skills) :
   ```
   /bmad-document-project → /bmad-generate-project-context
   /bmad-agent-pm → /bmad-edit-prd → /bmad-validate-prd
   /bmad-agent-ux-designer → /bmad-create-ux-design
   /bmad-agent-architect → /bmad-create-architecture
   /bmad-agent-pm → /bmad-create-epics-and-stories
   /bmad-agent-architect → /bmad-check-implementation-readiness
   /bmad-agent-dev → /bmad-sprint-planning
   ```

4. **Dev loop** — pour chaque story ready-for-dev dans
   `sprint-status.yaml` :
   ```
   /bmad-create-story → /bmad-dev-story → /bmad-qa-generate-e2e-tests
   /bmad-code-review
   ```
   BMAD gère lui-même : branch feature par story, commit, push,
   ouverture de PR via `gh`, mise à jour du sprint status.

5. **Fin d'epic** → `/bmad-agent-dev` + `/bmad-retrospective`
   automatiquement.

6. **Nouvelle itération** — sur un projet livré, un bouton
   « ➕ Nouvelle itération » ouvre un chat séparé pour ajouter une
   feature. Relance l'IterationPipeline (brownfield) sans toucher
   aux stories done.

---

## Dépendances

- **Go 1.25+** — pour builder Hive.
- **Node 20+** — pour installer BMAD (`npx bmad-method install`).
- **Claude Code CLI** sur le PATH — `claude --version`.
- **`gh` CLI** (optionnel, recommandé) — pour cloner/créer des repos
  GitHub depuis l'UI et pour que BMAD ouvre les PRs de stories.

---

## Démarrage

```bash
# Build + dashboard embarqué dans le binaire
make build

# Ou dev loop avec HMR côté front + air côté backend
make dev     # Vite sur :5173, hive sur :8233
make serve   # build complet puis ./hive serve
```

Ouvre <http://localhost:8233> et crée ton premier projet. L'intégration
GitHub se fait via un token personnel collé dans l'UI (login PAT) —
scopes recommandés : `repo`, `workflow`, `read:org`.

---

## Architecture

| Couche | Emplacement | Rôle |
|---|---|---|
| API REST + WS | `internal/api/` | Routes `/api/v1/projects/*`, `/intake/*`, `/iterate/*`, `/phases`, `/cancel`, `/retry-architect`, `/fs/*`, `/gh/*` + hub WebSocket |
| BMAD runner | `internal/bmad/` | `Install()` (npx bmad-method), `Invoke()` (`claude --print --dangerously-skip-permissions`), `RunSequenceObserved()` (liste de slash-commands + callbacks progress/cost) |
| Workflow BMAD | `internal/bmad/workflow.go` | Les 6 séquences officielles (Analysis, Planning, Solutioning, ImplementationInit, Story, Review, Retrospective) + FullPlanningPipeline et IterationPipeline |
| Dev loop | `internal/devloop/` | Supervisor polling `building` projects, parallélisme borné (3 projets), invocation des skills dev/review BMAD per story, crash recovery |
| Project store | `internal/project/` | CRUD + epic/story/AC tree sur SQLite |
| Intake | `internal/intake/` | Conversation PM avec rubric scripted (agent fallback), `IterationAgent` pour brownfield |
| Git | `internal/git/`, `internal/devloop/git.go` | Clone/create repo via `gh`, auth status, local git init pour le bootstrap |
| Dashboard | `web/` | SvelteKit statique embarqué dans le binaire Go |

---

## Mode dégradé

Quand la CLI `claude` n'est pas dispo, Hive bascule automatiquement
sur `ScriptedDev` + `ScriptedReviewer` — agents déterministes qui
écrivent un notes file par story. Utilisé pour :
- CI sans crédits Claude
- Smoke-tests locaux (`HIVE_DEV_AGENT=scripted hive serve`)
- Garder un flow end-to-end fonctionnel quand Claude est down

Idem quand `gh` n'est pas installé/authentifié : Hive tourne en mode
local-only (commits locaux, pas de PR).

---

## Tests end-to-end

Script qui lance une vraie build BMAD contre Claude + gh :

```bash
HIVE_E2E_TIMEOUT=3600 ./scripts/claude-e2e.sh
```

Le script instancie un hive jetable sur :18233, crée un mini projet
(« CLI qui sort un compliment aléatoire »), laisse BMAD tourner son
pipeline complet et vérifie que le projet flippe en `shipped`.
Observable dans le dashboard à `http://localhost:18233` pendant
l'exécution.

---

## Licence

Apache-2.0.

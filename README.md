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
GitHub se fait soit via un token personnel collé dans l'UI (login PAT,
scopes recommandés : `repo`, `workflow`, `read:org`), soit via OAuth
device flow en cliquant « Se connecter via navigateur ».

Pour recevoir des notifications Slack sur les événements critiques
(ship, échec pipeline, plafond coût atteint) :

```bash
export HIVE_SLACK_WEBHOOK=https://hooks.slack.com/services/...
./hive serve
```

Testable depuis `/settings` avec le bouton « Tester le webhook ».

---

## Dashboard

- `/` — accueil, état global
- `/projects` — liste + création de projets (greenfield / brownfield)
- `/projects/{id}` — détail : phases BMAD live, coût cumulé, édition
  intake, bouton **Reprendre au step suivant** (saute les skills déjà
  réussies) vs **Relancer BMAD** (tout recommencer), export `.tar.gz`
- `/costs` — consommation Claude agrégée : par projet / par phase /
  top commandes, projection coût/h, export CSV
- `/events` + `/audit` — observabilité temps réel
- `/settings` — état des notifications (webhook Slack) + bouton test

Endpoints d'intégration hors-UI :
- `GET /metrics` — exposition Prometheus
- `GET /api/openapi.yaml` — spec OpenAPI 3.1 complète

---

## Architecture

| Couche | Emplacement | Rôle |
|---|---|---|
| API REST + WS | `internal/api/` | Routes `/api/v1/projects/*`, `/intake/*`, `/iterate/*`, `/phases`, `/cancel`, `/retry-architect` (avec `?from_step=N`), `/fs/*`, `/gh/*`, `/costs`, `/settings/notify/*` + hub WebSocket |
| BMAD runner | `internal/bmad/` | `Install()` (npx bmad-method), `Invoke()` (`claude --print --dangerously-skip-permissions`), `RunSequenceObserved()` (liste de slash-commands + callbacks progress/cost). Coverage 74%. |
| Workflow BMAD | `internal/bmad/workflow.go` | Les séquences officielles (Analysis, Planning, Solutioning, ImplementationInit, Story, Review, Retrospective) + FullPlanningPipeline et IterationPipeline |
| Dev loop | `internal/devloop/` | Supervisor polling `building` projects, parallélisme borné (3 projets), invocation des skills dev/review BMAD per story, crash recovery, fallback git direct quand BMAD ne pousse pas de PR |
| Project store | `internal/project/` | CRUD + epic/story/AC tree sur SQLite |
| Intake | `internal/intake/` | Conversation PM avec rubric scripted (agent fallback), `IterationAgent` pour brownfield |
| Git | `internal/git/` | Clone/create repo via `gh`, OAuth device flow, PAT login, `EnsureStoryPushed` (commit+push+PR idempotent) |
| Notifications | `internal/notify/` | Webhook Slack opt-in via `HIVE_SLACK_WEBHOOK`, événements : `project.shipped`, `*_failed`, `cost_cap_reached` |
| Métriques | `internal/metrics/` | Prometheus counters + histograms exposés sur `GET /metrics` (requests, skill cost, durations, events) |
| OpenAPI | `internal/api/openapi.yaml` | Spec 3.1 complète des ~30 routes, servie sur `GET /api/openapi.yaml` |
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

## Rotation des logs

Hive écrit tous ses logs sur stderr (`slog.NewTextHandler` au format
text). Pas de rotation in-process — le runtime Unix le fait mieux.

### Avec systemd (reco production)

```ini
[Service]
ExecStart=/usr/local/bin/hive serve
StandardOutput=append:/var/log/hive/hive.log
StandardError=append:/var/log/hive/hive.log
```

puis config `logrotate` :

```
/var/log/hive/hive.log {
    daily
    rotate 14
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
}
```

### Avec un simple nohup

```bash
nohup hive serve >> /var/log/hive/hive.log 2>&1 &
```

puis le même bloc `logrotate` avec `copytruncate` (Hive ne supporte pas
`SIGHUP` pour réouvrir le FD, donc copytruncate est le bon choix).

### Dev local

Pas besoin. `hive serve` affiche les logs dans le terminal comme
prévu ; redirige vers un fichier si tu veux les garder.

---

## Tests end-to-end

Deux scripts qui lancent une vraie build BMAD contre Claude + gh :

```bash
# Greenfield : crée un nouveau projet « CLI compliment aléatoire »
HIVE_E2E_TIMEOUT=3600 ./scripts/claude-e2e.sh

# Brownfield : clone un repo existant et ajoute une feature (--version)
HIVE_BROWNFIELD_REPO=owner/repo ./scripts/claude-e2e-brownfield.sh
```

Chaque script instancie un hive jetable, drive intake → finalize →
attend `shipped`. Observable dans le dashboard pendant l'exécution.
Compte ~$5–15 en tokens Claude par run (ou équivalent quota Pro/Max
si tu tournes via Claude Code en subscription).

En dev sans claude : `HIVE_DEV_AGENT=scripted ./hive serve` pour
exercer le flow avec les agents déterministes fallback.

---

## Licence

Apache-2.0.

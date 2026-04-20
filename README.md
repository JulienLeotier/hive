# Hive

**Usine à produits BMAD en local.** Tu décris une idée, Hive lance le
framework [BMAD-METHOD](https://github.com/bmad-code-org/BMAD-METHOD) de
bout en bout via Claude Code — PRD, architecture, stories, implémentation,
revue de code, rétrospective — jusqu'à ce que le produit soit livré.

Un seul binaire Go, un dashboard SvelteKit, une base SQLite. Tout se
pilote depuis le navigateur.

**Nouveau sur Hive ?** → [`docs/walkthrough.md`](docs/walkthrough.md)
pour un guide pas-à-pas de l'idée au projet `shipped`, avec les
actions manuelles disponibles et le troubleshooting.

---

## Flow

1. **Nouveau projet** (`/projects`) — tu décris l'idée. Optionnel :
   clone un repo GitHub existant ou pointe un repo local (mode
   brownfield). Un `cost_cap_usd` optionnel coupe automatiquement le
   pipeline si le cumul Claude dépasse ce montant.

2. **Intake PM** — un agent te pose des questions (audience, flows,
   non-goals, tech, DoD) en une ligne à la fois. Le PM est ancré pour
   **respecter l'ambition** de l'utilisateur : si tu dis « simple
   todolist », il ne propose pas de framework lourd. Il produit un
   brief SCOPE LOCKED que BMAD doit respecter ensuite.

3. **Finalisation** → Hive écrit le brief dans `_bmad-output/
   planning-artifacts/product-brief-<slug>.md` puis lance la séquence
   BMAD officielle, une skill par invocation `claude --print` :

   **Greenfield** (11 skills, on bypass `/bmad-agent-analyst` +
   `/bmad-product-brief` car le PM Hive écrit déjà le brief) :
   ```
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

4. **Dev loop** — un supervisor polle les projets `building` toutes
   les 10 s (override : `HIVE_DEVLOOP_INTERVAL`). Pour chaque story
   `pending` :
   ```
   /bmad-create-story → /bmad-dev-story → /bmad-qa-generate-e2e-tests
   /bmad-code-review
   ```
   Si le reviewer tag un finding `decision-needed`, Hive escalade
   automatiquement à `/bmad-agent-architect` + `/bmad-correct-course`
   sans consommer le budget d'itérations (cappé à 2 escalations par
   story). Au-delà de 3 itérations classiques, la story passe en
   `blocked` et l'UI propose un bouton **↻ Réessayer**.

5. **Fin d'epic** → `/bmad-agent-dev` + `/bmad-retrospective`
   automatiquement.

6. **Nouvelle itération** — sur un projet livré, un bouton
   « ➕ Nouvelle itération » ouvre un chat séparé pour ajouter une
   feature. Relance l'IterationPipeline (brownfield) sans toucher
   aux stories done.

---

## Dépendances

- **Go 1.26+** — pour builder Hive (CI pinned à 1.26).
- **Node 20+** — pour installer BMAD (`npx bmad-method install`).
- **Claude Code CLI** sur le PATH — `claude --version` (Hive log la
  version détectée au boot et l'affiche dans `/settings`).
- **`gh` CLI** (optionnel, recommandé) — pour cloner/créer des repos
  GitHub depuis l'UI et pour que BMAD ouvre les PRs de stories. Sans
  `gh`, Hive reste en mode local-only.

---

## Démarrage

```bash
# Build + dashboard embarqué dans le binaire
make build

# Ou dev loop avec HMR côté front + air côté backend
make dev     # Vite sur :5173, hive sur :8233
make serve   # build complet puis ./hive serve
```

Ouvre <http://localhost:8233> et crée ton premier projet.

**Intégration GitHub** — soit via un token personnel collé dans l'UI
(login PAT, scopes recommandés : `repo`, `workflow`, `read:org`), soit
via OAuth device flow en cliquant « Se connecter via navigateur ».

**Notifications Slack** :

```bash
export HIVE_SLACK_WEBHOOK=https://hooks.slack.com/services/...
./hive serve
```

Testable depuis `/settings` avec le bouton « Tester le webhook ».

**Sauvegarde SQLite** :

```bash
hive backup hive-2026-04-20.tar.gz   # VACUUM INTO + .tar.gz atomique
hive restore hive-2026-04-20.tar.gz  # refuse d'écraser sans --force
```

---

## Dashboard

- `/` — accueil avec grille des projets actifs (status, coût, paused)
- `/projects` — liste + filtre client-side + création (greenfield /
  brownfield)
- `/projects/{id}` — détail en onglets :
  - **Vue d'ensemble** : pipeline BMAD live (clic sur un step →
    drawer console avec les events stream-json parsés en badges
    typés), barre budget vs cap, actions (retry, cancel per-step,
    rerun)
  - **Stories** : arbre epics → stories → ACs avec verdicts
    individuels + bouton `⚡ Skill BMAD` pour lancer un skill manuel
    sur une story précise (code-review, correct-course, etc.)
  - **PRD** : édition inline + régénération
  - **Activité** : feed WebSocket temps réel
- `/costs` — consommation Claude agrégée : par projet / par phase /
  top commandes, projection coût/h, export CSV
- `/events` + `/audit` — observabilité temps réel
- `/settings` — notifications Slack, stats DB, bouton « Nettoyer
  maintenant » (retention events > 90j + audit > 365j), maintenance
  groupée (delete failed, unwedge stories), version Claude CLI
  détectée, variables d'env utiles
- `/api-docs` — toutes les routes REST listées depuis l'OpenAPI
  (pas de CDN externe, tout local)

**Raccourcis clavier** :
- `/` ou `Cmd/Ctrl + K` — palette de recherche globale (projets,
  epics, stories)
- `g` puis `p / e / h / s` — navigation vers `/projects`, `/events`,
  `/`, `/settings`
- `Esc` — ferme modals et drawers
- `Enter` — confirme dans les modals

**Endpoints d'intégration hors-UI** :
- `GET /metrics` — exposition Prometheus
- `GET /api/openapi.yaml` — spec OpenAPI 3.1 complète
- `GET /api/v1/projects/{id}/report.md` — rapport Markdown (PRD +
  plan + ACs + reviews + coût par phase)

---

## Architecture

| Couche | Emplacement | Rôle |
|---|---|---|
| API REST + WS | `internal/api/` | Handlers séparés par thème : `projects.go`, `intake.go` (chat), `intake_pipeline.go` (BMAD async), `intake_observer.go` (stepObserver/trackedInvoke), `admin.go` (sweep, bulk delete, unwedge), `search.go`, `report.go`, `bmad_skills.go`, `host_guard.go` (DNS rebinding defense) |
| BMAD runner | `internal/bmad/` | `Install()`, `Invoke()` + `InvokeStream()` (stream-json + dead-man timeout `HIVE_SKILL_TIMEOUT`), `RunSequenceObserved()` (per-step child ctx pour cancel chirurgical), registry `skills.go`, process group pour tuer claude + sous-process |
| Workflow BMAD | `internal/bmad/workflow.go` | Les séquences officielles (`PlanningSequence`, `SolutioningSequence`, `StorySequence`, `ReviewSequence`, `RetrospectiveSequence`, `ArchitectEscalationSequence`) + `FullPlanningPipeline` et `IterationPipeline` |
| Dev loop | `internal/devloop/` | Supervisor polling `building` projects, gate anti-ticks-concurrents (1 advance par projet à la fois), parallélisme borné (3 projets), escalation architect autonome, crash recovery, fallback `EnsureStoryPushed` |
| Project store | `internal/project/` | CRUD + epic/story/AC tree sur SQLite, `paused` flag pour mettre un projet en pause après cancel |
| Intake | `internal/intake/` | Conversation PM ancrée anti-drift (STACK LOCK + ambition matching), `ScriptedAgent` fallback, `IterationAgent` pour brownfield |
| Git | `internal/git/` | Clone/create repo via `gh`, OAuth device flow, PAT login, `EnsureStoryPushed` (auto-crée `feat/<slug>` si BMAD oublie), `audit.go` (snapshot pre/post-skill pour détecter commits sur main) |
| Notifications | `internal/notify/` | Webhook Slack opt-in via `HIVE_SLACK_WEBHOOK` |
| Métriques | `internal/metrics/` | Prometheus counters + histograms sur `GET /metrics` |
| OpenAPI | `internal/api/openapi.yaml` | Spec 3.1 complète, servie sur `GET /api/openapi.yaml` + rendue en UI sur `/api-docs` |
| Dashboard | `web/` | SvelteKit statique embarqué. Tests Vitest (`web/src/lib/*.test.ts`) |

---

## Variables d'environnement

| Variable | Défaut | Effet |
|---|---|---|
| `HIVE_DEV_AGENT` | `claude-code` | `scripted` force les agents déterministes (CI, debug sans Claude) |
| `HIVE_DEVLOOP_INTERVAL` | `10s` | Cadence du supervisor dev loop |
| `HIVE_MAX_PARALLEL_PROJECTS` | `3` | Nb de projets `building` avancés en parallèle |
| `HIVE_SKILL_TIMEOUT` | `0` (pas de cap) | Dead-man timeout par skill (ex `45m`) |
| `HIVE_SLACK_WEBHOOK` | — | URL webhook Slack pour les notifications critiques |
| `HIVE_EXTRA_HOSTS` | — | Hosts additionnels acceptés par le DNS-rebinding guard (CSV) |
| `HIVE_INTAKE_AGENT` | `claude-code` | `scripted` pour forcer le PM déterministe |

---

## Sécurité défensive

Hive est un outil local single-user, mais ajoute les gardes suivantes :

- **DNS rebinding guard** — Host header validé contre une allowlist
  (localhost, 127.0.0.1, IPs RFC1918, `*.local`, + `HIVE_EXTRA_HOSTS`).
  Un site malveillant qui résout `evil.example` → 127.0.0.1 ne peut
  pas hit l'API depuis le browser de la victime.
- **Process group kill** — chaque `claude --print` tourne dans son
  propre pgid ; sur shutdown/cancel, SIGKILL au groupe entier tue
  aussi les sous-process (node, python, bash lancés par tool_use).
- **Per-step cancel chirurgical** — ctx-enfant par skill BMAD,
  registre keyed par `phase_step.id`, endpoint
  `POST /api/v1/phases/{id}/cancel` qui tue UNE skill précise sans
  toucher au reste du projet.
- **Git audit pre/post-skill** — snapshot HEAD + branch avant/après
  chaque invocation ; si BMAD a commité directement sur `main` au
  lieu d'une feat branch, l'UI annote l'étape avec un ⚠ visible.
- **Canari stream-json** — si le CLI claude renvoie 0 event
  parseable, Hive lève une erreur explicite au lieu d'un faux OK.
- **Pause automatique après cancel** — le devloop skippe les projets
  `paused=1` jusqu'à ce que l'opérateur clique « Reprendre ». Plus
  de rattrapage surprise 10 s après un cancel.

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

Puis le même bloc `logrotate` avec `copytruncate` (Hive ne supporte
pas `SIGHUP` pour réouvrir le FD).

### Dev local

Pas besoin. `hive serve` affiche les logs dans le terminal comme
prévu ; redirige vers un fichier si tu veux les garder.

---

## Tests

### Backend (Go)

```bash
go test ./...                      # full suite
go test -race -cover ./internal/... # avec coverage
```

CI gate la coverage core (bmad + devloop + event + intake + project)
à **55%** (réelle : ~60%).

### Frontend (Vitest)

```bash
cd web
npm test         # run une fois
npm run test:watch
```

### End-to-end (contre Claude réel)

```bash
# Greenfield : crée un nouveau projet « CLI compliment aléatoire »
HIVE_E2E_TIMEOUT=3600 ./scripts/claude-e2e.sh

# Brownfield : clone un repo existant et ajoute une feature
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

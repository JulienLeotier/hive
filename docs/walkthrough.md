# Walkthrough — de l'idée au projet `shipped`

Ce guide accompagne un premier projet BMAD de bout en bout. Suppose
que `hive serve` tourne déjà (voir `README.md` pour l'install).

## 1. Créer un projet

`http://localhost:8080/projects` → bouton **Nouveau projet**.

- **Nom** — court, descriptif (`todolist`, `crm-leads`). Hive l'utilise
  pour les noms de fichiers générés et les labels UI.
- **Idée** — une ligne décrivant ce que tu veux. L'agent PM va te
  questionner pour raffiner, donc l'idée initiale peut être vague.
- **Répertoire de travail** (workdir) — où BMAD va écrire le code.
  Crée un sous-dossier dédié (ex. `~/projets/todolist`). Hive refuse
  les dossiers qui contiennent déjà des fichiers personnels
  (Documents, Downloads, etc.) pour éviter de `git add -A` tes
  photos.
- **Intégration GitHub** (optionnelle) — clone un repo existant pour
  une itération brownfield, crée un repo neuf via `gh`, ou laisse à
  vide pour un projet strictement local.

Le projet est créé en statut `draft`. Le PM agent t'accueille dans le
chat d'intake.

## 2. Intake chat (Phase 1)

Le PM agent pose une question à la fois. Réponds naturellement. Sa job :
extraire audience, core flows, non-goals, stack, definition-of-done.

**Mots-clés qui l'aident** :
- `simple`, `basic`, `minimal`, `just X` → il garde un scope tight
  (vanilla HTML/JS/localStorage par défaut pour du web simple, pas
  de framework)
- `utilise ta meilleure idée` / `débrouille-toi` → il pose les
  defaults explicitement et handoff

Quand il a assez de contexte il marque `done=true`. Clique
**Finalize PRD** → bascule en `planning`.

## 3. Planning BMAD (Phase 2 + 3)

Le backend lance en tâche de fond :

```
/bmad-agent-pm → /bmad-create-prd → /bmad-validate-prd
→ /bmad-agent-ux-designer → /bmad-create-ux-design
→ /bmad-agent-architect → /bmad-create-architecture
→ /bmad-agent-pm → /bmad-create-epics-and-stories
→ /bmad-agent-architect → /bmad-check-implementation-readiness
→ /bmad-agent-dev → /bmad-sprint-planning
```

Chaque skill apparaît dans l'onglet **Vue d'ensemble** → section
**Pipeline BMAD**. Clique une ligne pour ouvrir la **console Claude**
en live (events stream-json parsés).

**Coût typique** : $2-5 pour un projet simple, $5-15 pour un projet
complexe. Tu peux poser un cap via `cost_cap_usd` dans l'API.

## 4. Devloop (Phase 4)

Le projet passe en `building`. Le supervisor tick toutes les 10s :
- Pick la prochaine story `pending`
- Lance `/bmad-create-story` → `/bmad-dev-story` → `/bmad-code-review`
- Valide les ACs une par une
- Si pass → story `done`, passe à la suivante
- Si fail → retry (max 3 itérations) puis `blocked`
- Si le reviewer flag `decision-needed` → `/bmad-agent-architect` +
  `/bmad-correct-course` (max 2 escalations puis cap)

### Actions manuelles

Depuis l'UI, tu peux à tout moment :

- **⚡ Skill BMAD** (dropdown) — lancer n'importe quel skill sur le
  projet (`validate-prd`, `sprint-planning`…) ou sur une story
  (`code-review`, `correct-course`). Utile quand la boucle autonome
  dérive.
- **↻ Relancer un step** dans l'historique phases — rejoue un skill
  précis sans relancer tout le pipeline.
- **✕ Annuler ce skill** — tue le skill en vol ET pause le projet.
  Le devloop ne repique pas automatiquement. Clique **Reprendre**
  pour redémarrer.
- **Retry story** (sur story blocked) — reset le compteur à 0.

## 5. Shipped + itérations

Quand toutes les stories sont `done`, le projet flippe en `shipped`.
Tu peux :

- **Nouvelle itération** — relance le PM en mode brownfield pour
  ajouter une feature. `/bmad-document-project` scan le code existant,
  `/bmad-edit-prd` étend le PRD, et on recommence la boucle dev/review
  sur les nouvelles stories uniquement.
- **Rétrospective** — `/bmad-agent-dev` + `/bmad-retrospective` génère
  un post-mortem basé sur l'historique.
- **Rapport Markdown** (`▤ Rapport .md`) — un artefact Markdown
  complet (PRD + plan + ACs + reviews + coût par phase) pour archiver.

## 6. Troubleshooting

| Symptôme | Cause probable | Action |
|---|---|---|
| Spinner infini en `planning`, pas de banner | Serveur redémarré pendant le pipeline | Le projet est flag `interrupted` au boot — la banner **Reprendre au step suivant** apparaît |
| Une skill reste en `running` plusieurs heures | Claude CLI planté ou hung | **✕ Annuler ce skill** puis **Reprendre** |
| Le reviewer boucle sur les mêmes findings | BMAD dérive de la story | Lance `/bmad-correct-course` en manuel pour recadrer |
| Le projet consomme trop | Pipeline répété inutilement | Pose un `cost_cap_usd` sur le projet — Hive abort auto |
| Les ACs restent toutes ○ malgré review | Reviewer marque tout en fail global | Relance **Code Review** en manuel et check la console pour voir les patterns `AC<N>: ✓` attendus par le parser |

## 7. Réglages utiles

- `HIVE_DEV_AGENT=scripted` — force le devloop en mode déterministe
  (aucun appel Claude). Pour CI ou pour tester Hive lui-même.
- `HIVE_DEVLOOP_INTERVAL=30s` — ralentit le tick si tu veux souffler.
- `HIVE_SLACK_WEBHOOK=...` — notifs des events `story.blocked`,
  `project.shipped`, etc.
- **Settings → Base de données → Nettoyer maintenant** — vide les
  events (> 90j) et audit_log (> 365j). Utile quand la DB grossit.

# Politique de sécurité

## Versions supportées

Hive n'a pas de cycle de release formel. **La branche `main`** est la
seule version supportée — elle reçoit les correctifs de sécurité.
Toute version antérieure (tags, forks) doit rebase sur `main` pour
bénéficier des fixes.

## Signaler une vulnérabilité

**Ne pas ouvrir d'issue GitHub publique pour une vuln critique.**
Contacte l'owner :

- GitHub : [@JulienLeotier](https://github.com/JulienLeotier)
- Via la feature [Private vulnerability reporting](https://github.com/JulienLeotier/hive/security/advisories/new)
  (préférée).

Mentionne :
- **Vecteur** (comment tu as déclenché le comportement)
- **Impact** (ce que peut faire l'attaquant)
- **Reproduction minimale**
- **Commit SHA** concerné
- **Proposition de patch** si tu en as une

## Modèle de menace

Hive est un outil **local, single-user**. Les garanties :

- ✅ Le serveur HTTP binde par défaut sur `localhost:8233`
- ✅ `HostGuard` bloque les Host headers non-allowlist (défense DNS
  rebinding)
- ✅ `SecurityHeaders` pose X-Frame-Options, CSP strict, HSTS sur TLS
- ✅ `rateLimit` cappe les requêtes distantes (localhost exempt)
- ✅ `claude --print` tourne en pgid isolé, SIGKILL au groupe entier
  sur cancel/shutdown
- ✅ Backup atomique via `VACUUM INTO` + tar.gz path-traversal-safe

**Hors scope (intentionnel) :**

- ❌ Auth multi-user : Hive injecte un admin `default` à chaque
  requête. Ne JAMAIS exposer Hive sur un port public sans reverse
  proxy + auth (nginx + basic-auth, Cloudflare Access, Tailscale…).
- ❌ Secrets management : les tokens GitHub passent via `gh`, les
  webhooks Slack via env var. Pas de vault intégré.
- ❌ Sandboxing BMAD : Claude Code tourne avec
  `--dangerously-skip-permissions` dans le workdir. Un workdir
  choisi à la légère peut exfiltrer / modifier des fichiers
  personnels (une allowlist dans `internal/git/git.go` refuse les
  home directories et autres dossiers sensibles, mais c'est un
  garde-fou, pas un sandbox).

## Scope accepté pour un rapport

- Bypass de `HostGuard`
- Injection SQL / XSS dans les endpoints
- Path traversal via `FSList`, `FSMkdir`, backup/restore, export
- Command injection via les inputs PATCH/POST (workdir, name, etc.)
- Fuite de tokens dans les logs / responses / metrics
- Vulnérabilité dans les dépendances Go ou npm (govulncheck et
  `npm audit --audit-level=high` tournent en CI — un high/critical
  manqué est un bug)

## Hors scope

- Attaque nécessitant déjà un accès shell à la machine hôte
- Déni de service localhost (Hive est single-user, l'utilisateur peut
  juste killer le process)
- Bugs fonctionnels sans implication sécurité (utiliser un bug
  report classique)
- Absence d'auth (cf. modèle de menace)

Merci pour ta vigilance.

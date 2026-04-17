-- Migration 025 : drop les tables pre-pivot jamais lues ni ecrites.
-- Ces tables ont ete creees par les migrations 001-016 pour la
-- plateforme multi-agents remplacee par le produit BMAD single-user.
-- Le code a ete supprime en P43, les tables restaient car dropper
-- est irreversible. Apres verif aucun handler ne les reference.
-- Idempotent via DROP TABLE IF EXISTS.

DROP TABLE IF EXISTS agent_tokens;
DROP TABLE IF EXISTS agent_trust_overrides;
DROP TABLE IF EXISTS agents;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS auctions;
DROP TABLE IF EXISTS bids;
DROP TABLE IF EXISTS budget_alerts;
DROP TABLE IF EXISTS cluster_members;
DROP TABLE IF EXISTS costs;
DROP TABLE IF EXISTS dialog_messages;
DROP TABLE IF EXISTS dialog_threads;
DROP TABLE IF EXISTS federation_links;
DROP TABLE IF EXISTS invoices;
DROP TABLE IF EXISTS knowledge;
DROP TABLE IF EXISTS optimizations;
DROP TABLE IF EXISTS rbac_users;
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS trust_history;
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhooks;
DROP TABLE IF EXISTS workflows;

-- Garde-fou budget : cost_cap_usd est le plafond de dépenses Claude
-- autorisé pour un projet. Dès que total_cost_usd le dépasse, le
-- superviseur annule le pipeline (status → failed, stage →
-- cost-cap). Zéro ou NULL = pas de plafond.
ALTER TABLE projects ADD COLUMN cost_cap_usd REAL DEFAULT 0;

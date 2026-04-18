-- Flag de pause par projet. Quand l'opérateur clique "Annuler" sur une
-- skill en cours, on set paused=1. Le devloop filtre paused=1 dans
-- buildingProjects() donc aucun nouveau tick ne relance de dev-story
-- tant que l'opérateur ne clique pas "Reprendre".
--
-- Sans ce flag, le devloop re-piquait la story au prochain tick
-- (10s plus tard) — l'opérateur voyait ses cancels se faire
-- immédiatement remplacés par de nouveaux skills. Frustration.
ALTER TABLE projects ADD COLUMN paused INTEGER NOT NULL DEFAULT 0;

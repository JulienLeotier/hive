-- is_existing = 1 pour les projets "brownfield" : un repo existant
-- cloné ou pointé via repo_path. Hive lance alors IterationPipeline
-- (bmad-document-project + bmad-edit-prd + ...) au lieu de
-- FullPlanningPipeline (création from scratch).
ALTER TABLE projects ADD COLUMN is_existing INTEGER NOT NULL DEFAULT 0;

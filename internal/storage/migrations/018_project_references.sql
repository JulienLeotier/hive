-- Let projects reference an existing BMAD output directory and an existing
-- git repository. When the operator brings either, the autonomous flow
-- skips the corresponding phase:
--   - bmad_output_path set = Architect skips decomposition, reads the
--     epics/stories already authored
--   - repo_path set = Dev agents work inside that repo instead of
--     scaffolding a fresh one, which makes Hive usable for "add feature X
--     to my existing codebase" and not just greenfield builds.
ALTER TABLE projects ADD COLUMN bmad_output_path TEXT;
ALTER TABLE projects ADD COLUMN repo_path TEXT;

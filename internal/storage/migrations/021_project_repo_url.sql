-- Hive peut maintenant cloner ou créer le repo GitHub du projet
-- depuis l'interface (via la CLI `gh`). On stocke l'URL canonique
-- pour linker dans le dashboard et fournir la source of truth au
-- workflow brownfield BMAD.
ALTER TABLE projects ADD COLUMN repo_url TEXT;

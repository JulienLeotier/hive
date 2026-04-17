-- Each story now runs on its own branch. When the dev agent pushes
-- and opens a pull request (github via gh CLI, when a remote is
-- configured), we stash the PR URL here so the dashboard can link to
-- it and the reviewer knows where to post comments on subsequent
-- iterations.
ALTER TABLE stories ADD COLUMN pr_url TEXT;

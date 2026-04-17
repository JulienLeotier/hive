-- Agent quota: max concurrent tasks. 0 = unlimited (existing behaviour).
-- The router uses this to skip saturated agents so a fleet can't pile on
-- a single slow adapter and starve the queue.
ALTER TABLE agents ADD COLUMN max_concurrent INTEGER NOT NULL DEFAULT 0;

-- Agent quota: max concurrent tasks. 0 = unlimited (existing behaviour).
ALTER TABLE agents ADD COLUMN IF NOT EXISTS max_concurrent INTEGER NOT NULL DEFAULT 0;

-- Mirror of the SQLite 026 — add full console reply column so the UI
-- can show what Claude actually said, not just the 600-char preview.
ALTER TABLE bmad_phase_steps ADD COLUMN IF NOT EXISTS reply_full TEXT;

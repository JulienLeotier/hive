-- Full console reply per BMAD skill invocation. Before this column we
-- only kept reply_preview (600 first chars) which was fine for the
-- phases list but useless when the operator wanted to see WHY a skill
-- drifted or what Claude actually said. The UI now shows a "Console"
-- panel per step that reads reply_full directly.
--
-- Kept separate from reply_preview so the list view stays lightweight:
-- no need to ship megabytes when you're just rendering a status badge.
ALTER TABLE bmad_phase_steps ADD COLUMN reply_full TEXT;

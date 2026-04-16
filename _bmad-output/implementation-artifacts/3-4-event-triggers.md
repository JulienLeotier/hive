# Story 3.4: Event Triggers

Status: done

## Story

As a user,
I want workflows triggered by events (webhooks, schedules, or manual),
so that my hive reacts automatically to external signals.

## Acceptance Criteria

1. **Given** a workflow with a trigger definition in `hive.yaml` **When** the trigger section is parsed **Then** the trigger type (manual, webhook, schedule) and configuration are stored in the TriggerDef struct
2. **Given** a trigger of type `manual` **When** the user runs `hive run` **Then** the workflow is instantiated and executed with the manual trigger
3. **Given** a trigger of type `schedule` **When** the cron expression matches **Then** the workflow is automatically instantiated (cron integration via heartbeat scheduler)
4. **Given** a trigger of type `webhook` **When** a matching HTTP request arrives **Then** the trigger payload is available as input to the first task (FR10)
5. **Given** the trigger configuration **When** it is parsed **Then** optional fields like `schedule` (cron) and `webhook` (endpoint path) are available

## Tasks / Subtasks

- [x] Task 1: Define TriggerDef struct with type, schedule, webhook fields (AC: #1, #5)
- [x] Task 2: Integrate trigger parsing into workflow Config YAML parser (AC: #1)
- [x] Task 3: Validate trigger configuration during parse (AC: #5)
- [x] Task 4: Write test for parsing workflow with trigger section (AC: #1)

## Dev Notes

- TriggerDef is embedded in the workflow Config struct with `yaml:"trigger,omitempty"`
- Three trigger types supported: `manual`, `webhook`, `schedule`
- Schedule trigger uses cron expression format (e.g., `*/5 * * * *`)
- Webhook trigger specifies an endpoint path for incoming HTTP requests
- The trigger definition is declarative -- actual trigger execution (cron scheduling, webhook routing) is handled at the server/scheduler level
- Manual trigger is the default when no trigger is specified (user runs `hive run`)
- TestParseWithTrigger validates the full trigger parsing flow

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### File List

- internal/workflow/parser.go (modified) -- TriggerDef struct, trigger parsing in Config
- internal/workflow/parser_test.go (modified) -- TestParseWithTrigger test

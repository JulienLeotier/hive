# Story 4.1: Agent Identity & Behavioral Plan Parser

Status: done

## Story

As a user,
I want to define agent identity (AGENT.yaml) and behavioral plans (PLAN.yaml),
so that each agent knows who it is and what to do autonomously.

## Acceptance Criteria

1. **Given** an `AGENT.yaml` file with identity, capabilities, constraints, and anti-patterns **When** the parser loads the file **Then** the agent's identity is stored in an AgentIdentity struct and used for all decision-making
2. **Given** the identity **When** constraints are defined **Then** they are available for enforcement (e.g., "never modify production data")
3. **Given** a `PLAN.yaml` file with a state machine definition **When** the parser loads the file **Then** the plan defines states, transitions, observation rules, action handlers, and idle conditions
4. **Given** the plan is modified by editing the YAML file **When** the agent next wakes up **Then** changes take effect (plan is re-parsed on demand)
5. **Given** an invalid plan (missing heartbeat, unknown initial state, invalid transitions) **When** the parser loads it **Then** clear error messages are produced (FR43, FR49, FR50)

## Tasks / Subtasks

- [x] Task 1: Define AgentIdentity struct with name, role, capabilities, constraints, anti_patterns (AC: #1, #2)
- [x] Task 2: Implement ParseIdentity to read and validate AGENT.yaml (AC: #1)
- [x] Task 3: Define Plan, StateDef, ActionDef, Transition structs (AC: #3)
- [x] Task 4: Implement ParsePlan and ParsePlanBytes for PLAN.yaml (AC: #3, #4)
- [x] Task 5: Implement validatePlan checking heartbeat, initial_state, states, transitions (AC: #5)
- [x] Task 6: Validate transition targets reference existing states (AC: #5)
- [x] Task 7: Write tests for identity parse, plan parse, missing fields, invalid transitions (AC: #1-#5)

## Dev Notes

- AgentIdentity captures the "who" (name, role, capabilities) and the "guardrails" (constraints, anti_patterns)
- Plan is a YAML-defined state machine: states contain observe rules, actions (when/do pairs), and transitions
- ActionDef.Do supports action types: `claim_task`, `idle`, `report_result`, `escalate`
- Validation is strict: heartbeat required, initial_state must exist in states list, all transition targets must be valid state names
- Both parsers use `gopkg.in/yaml.v3` with struct tags for clean YAML mapping
- The plan is designed to be human-editable -- operators can modify agent behavior by editing YAML
- ParsePlanBytes allows testing without filesystem access

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### File List

- internal/autonomy/plan.go (new) -- AgentIdentity, Plan, StateDef, ActionDef, Transition types; ParseIdentity, ParsePlan, ParsePlanBytes, validatePlan
- internal/autonomy/plan_test.go (new) -- 7 tests: identity parse, identity missing name, plan parse, missing heartbeat, missing initial_state, invalid initial_state, invalid transition target

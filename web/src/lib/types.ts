// Shared API response types.
//
// Until we generate types from an OpenAPI spec (deferred), these are a
// single source of truth consumed by every dashboard page. Each type
// mirrors the JSON shape returned under `data` by the matching endpoint.
//
// Keep comments next to fields that aren't obvious — frontend readers
// rarely have the Go handler open in another tab.

export type Agent = {
	id: string;
	name: string;
	type: string;
	health_status: string; // "healthy" | "degraded" | "unavailable"
	trust_level: string; // "scripted" | "supervised" | "guided" | "autonomous" | "trusted"
	capabilities: string; // JSON-encoded adapter capability blob
	updated_at: string;
};

export type Task = {
	id: string;
	workflow_id: string;
	type: string;
	status: string;
	agent_id: string;
	agent_name: string;
	created_at: string;
	duration_seconds?: number | null;
	result_summary?: string;
};

export type Workflow = {
	id: string;
	name: string;
	status: string;
	created_at: string;
};

export type Event = {
	id: number;
	type: string;
	source: string;
	payload: string;
	created_at: string;
};

export type AuditEntry = {
	id: number;
	action: string;
	actor: string;
	resource: string;
	detail: string;
	created_at: string;
};

export type ClusterMember = {
	node_id: string;
	hostname: string;
	address: string;
	status: string;
	last_heartbeat: string;
};

export type FederationLink = {
	name: string;
	url: string;
	status: string;
	shared_caps: string;
	last_heartbeat: string;
};

export type DialogThread = {
	id: string;
	initiator: string;
	participant: string;
	topic: string;
	status: string;
	message_count: number;
	created_at: string;
};

export type KnowledgeEntry = {
	id: number;
	task_type: string;
	approach: string;
	outcome: string;
	context: string;
	created_at: string;
};

export type Auction = {
	id: string;
	task_id: string;
	strategy: string;
	status: string;
	winner: string;
	bids: number;
	opened_at: string;
};

export type Recommendation = {
	type: string;
	description: string;
	impact: string;
	confidence: number;
};

export type AppliedOptimization = {
	id: string;
	setting: string;
	old_value: number;
	new_value: number;
	rationale: string;
	applied_at: string;
};

export type TrustPromotion = {
	id: string;
	agent: string;
	old_level: string;
	new_level: string;
	reason: string;
	criteria: string;
	created_at: string;
};

export type User = {
	Subject: string;
	Role: string;
	TenantID: string;
};

// /api/v1/metrics response. Shapes the home dashboard cards.
export type Metrics = {
	agents: { total: number; healthy: number; degraded: number; unavailable: number };
	tasks: Record<string, number>;
	workflows: Record<string, number>;
	circuit_breakers: { total: number; open: number };
	events: { last_minute: number; last_hour: number };
	avg_task_duration_seconds: number;
	timestamp: string;
};

// /api/v1/costs response envelope.
export type CostSummary = { agent_name: string; total_cost: number; task_count: number };
export type CostAlert = { agent_name: string; daily_limit: number; spend: number; breached: boolean };
export type CostWorkflowSummary = { workflow_id: string; total_cost: number; task_count: number };
export type CostTrendPoint = { day: string; total_cost: number };
export type CostsPayload = {
	summaries?: CostSummary[];
	alerts?: CostAlert[];
	per_workflow?: CostWorkflowSummary[];
	trend?: CostTrendPoint[];
};

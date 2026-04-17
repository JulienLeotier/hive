// Shared API response types.
//
// Only the shapes still in use by the BMAD dashboard are here. Pre-pivot
// types (Agent, Task, Workflow, Federation, Marketplace, Costs, etc.)
// were removed with their pages.

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

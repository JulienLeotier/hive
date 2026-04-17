<script lang="ts">
	import { fmtRelative, truncate } from '$lib/format';
	import { apiGet, apiPost, apiDelete } from '$lib/api';
	import { createReconnectingWS, wsURL } from '$lib/ws';
	import type { Agent } from '$lib/types';
	import ListScaffold from '$lib/ListScaffold.svelte';

	let agents = $state<Agent[]>([]);
	let loading = $state(true);

	// Create-form state
	let newName = $state('');
	let newType = $state('http');
	let newURL = $state('');
	let newMaxConcurrent = $state(0);
	let formError = $state('');
	let submitting = $state(false);

	async function loadAgents() {
		try {
			agents = (await apiGet<Agent[]>('/api/v1/agents')) ?? [];
		} catch {
			/* banner shown by apiGet */
		} finally {
			loading = false;
		}
	}

	async function createAgent(ev: Event) {
		ev.preventDefault();
		formError = '';
		submitting = true;
		try {
			await apiPost('/api/v1/agents', {
				name: newName,
				type: newType,
				url: newURL,
				max_concurrent: Number(newMaxConcurrent) || 0
			});
			newName = '';
			newURL = '';
			newMaxConcurrent = 0;
			await loadAgents();
		} catch (e) {
			formError = e instanceof Error ? e.message : String(e);
		} finally {
			submitting = false;
		}
	}

	async function removeAgent(name: string) {
		if (!confirm(`Remove agent "${name}"? In-flight tasks are requeued.`)) return;
		try {
			await apiDelete(`/api/v1/agents/${encodeURIComponent(name)}`);
			await loadAgents();
		} catch (e) {
			formError = e instanceof Error ? e.message : String(e);
		}
	}

	$effect(() => {
		loadAgents();
		const interval = setInterval(loadAgents, 10000);
		const ws = createReconnectingWS({
			url: wsURL('/ws'),
			onmessage: (msg) => {
				try {
					const evt = JSON.parse(msg.data);
					if (typeof evt.type === 'string' && evt.type.startsWith('agent.')) {
						loadAgents();
					}
				} catch {
					/* ignore non-JSON frames */
				}
			}
		});
		return () => {
			ws.close();
			clearInterval(interval);
		};
	});

	function statusColor(status: string): string {
		if (status === 'healthy') return 'var(--ok)';
		if (status === 'degraded') return 'var(--warn)';
		return 'var(--err)';
	}

	function summariseCaps(c: string): string {
		try {
			const parsed = JSON.parse(c);
			return (parsed.task_types ?? []).join(', ');
		} catch {
			return c;
		}
	}
</script>

<ListScaffold
	title="Agents"
	subtitle="Fleet registered on this hive. Real-time health via WebSocket."
	{loading}
	isEmpty={agents.length === 0}
	emptyText="No agents registered. Create one with the form below or use `hive add-agent`."
>
	<form class="create-form" onsubmit={createAgent}>
		<input placeholder="name" bind:value={newName} required />
		<select bind:value={newType}>
			<option value="http">http</option>
			<option value="claude-code">claude-code</option>
			<option value="mcp">mcp</option>
			<option value="crewai">crewai</option>
			<option value="langchain">langchain</option>
			<option value="autogen">autogen</option>
			<option value="openai">openai</option>
		</select>
		<input placeholder="https://agent.example.com" bind:value={newURL} required />
		<input
			type="number"
			min="0"
			placeholder="cap (0=∞)"
			bind:value={newMaxConcurrent}
			title="Max concurrent tasks for this agent (0 = use server default)"
		/>
		<button type="submit" disabled={submitting}>{submitting ? '…' : 'Register'}</button>
	</form>
	{#if formError}<div class="form-error">{formError}</div>{/if}
	<table>
		<thead>
			<tr>
				<th>Name</th><th>Type</th><th>Version</th><th>Health</th><th>Trust</th><th>Capabilities</th><th>Last check</th><th></th>
			</tr>
		</thead>
		<tbody>
			{#each agents as agent (agent.id)}
				<tr>
					<td><strong>{agent.name}</strong></td>
					<td><code>{agent.type}</code></td>
					<td><code>{agent.version ?? '1.0.0'}</code></td>
					<td>
						<span class="badge" style="background:{statusColor(agent.health_status)}">
							{agent.health_status}
						</span>
					</td>
					<td>{agent.trust_level}</td>
					<td>{truncate(summariseCaps(agent.capabilities), 60)}</td>
					<td>{agent.updated_at ? fmtRelative(agent.updated_at) : '—'}</td>
					<td><button class="row-del" onclick={() => removeAgent(agent.name)} title="Remove agent">✕</button></td>
				</tr>
			{/each}
		</tbody>
	</table>
</ListScaffold>

<style>
	.create-form {
		display: grid;
		grid-template-columns: 1fr 130px 2fr 110px auto;
		gap: 0.5rem;
		margin-bottom: 1rem;
		align-items: center;
	}
	.create-form input,
	.create-form select {
		padding: 0.4rem 0.6rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 4px;
		color: inherit;
		font: inherit;
	}
	.create-form button {
		padding: 0.4rem 1rem;
		background: var(--accent);
		color: white;
		border: none;
		border-radius: 4px;
		cursor: pointer;
		font-weight: 600;
	}
	.create-form button:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.form-error {
		padding: 0.5rem 0.75rem;
		background: rgba(240, 80, 80, 0.15);
		border-left: 3px solid var(--err);
		border-radius: 4px;
		color: var(--err);
		margin-bottom: 1rem;
		font-size: 0.85rem;
	}
	.row-del {
		padding: 0.2rem 0.45rem;
		background: transparent;
		color: var(--muted);
		border: 1px solid var(--border);
		border-radius: 3px;
		cursor: pointer;
		font-size: 0.8rem;
	}
	.row-del:hover {
		background: rgba(240, 80, 80, 0.15);
		color: var(--err);
		border-color: var(--err);
	}
</style>

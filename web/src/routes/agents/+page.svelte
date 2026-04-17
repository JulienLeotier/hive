<script lang="ts">
	import { fmtRelative, truncate } from '$lib/format';
	import { apiGet } from '$lib/api';
	import { createReconnectingWS, wsURL } from '$lib/ws';
	import type { Agent } from '$lib/types';

	let agents = $state<Agent[]>([]);
	let loading = $state(true);

	async function loadAgents() {
		try {
			agents = (await apiGet<Agent[]>('/api/v1/agents')) ?? [];
		} catch {
			/* banner shown by apiGet */
		} finally {
			loading = false;
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

<main>
	<h1>Agents</h1>
	<p class="subtitle">Fleet registered on this hive. Real-time health via WebSocket.</p>

	{#if loading}
		<div class="empty">Loading…</div>
	{:else if agents.length === 0}
		<div class="empty">No agents registered. Use <code>hive add-agent</code>.</div>
	{:else}
		<table>
			<thead>
				<tr>
					<th>Name</th><th>Type</th><th>Health</th><th>Trust</th><th>Capabilities</th><th>Last check</th>
				</tr>
			</thead>
			<tbody>
				{#each agents as agent (agent.id)}
					<tr>
						<td><strong>{agent.name}</strong></td>
						<td><code>{agent.type}</code></td>
						<td>
							<span class="badge" style="background:{statusColor(agent.health_status)}">
								{agent.health_status}
							</span>
						</td>
						<td>{agent.trust_level}</td>
						<td>{truncate(summariseCaps(agent.capabilities), 60)}</td>
						<td>{agent.updated_at ? fmtRelative(agent.updated_at) : '—'}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</main>

<style>
	.subtitle {
		color: var(--text-muted);
		margin-top: 0;
	}
</style>

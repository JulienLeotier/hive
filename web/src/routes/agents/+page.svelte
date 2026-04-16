<script lang="ts">
	type Agent = {
		id: string;
		name: string;
		type: string;
		health_status: string;
		trust_level: string;
		capabilities: string;
	};

	let agents = $state<Agent[]>([]);

	async function loadAgents() {
		try {
			const res = await fetch('/api/v1/agents');
			const json = await res.json();
			agents = json.data ?? [];
		} catch { /* API not ready */ }
	}

	$effect(() => {
		loadAgents();
		const interval = setInterval(loadAgents, 3000);
		return () => clearInterval(interval);
	});

	function statusColor(status: string): string {
		if (status === 'healthy') return '#22c55e';
		if (status === 'degraded') return '#f59e0b';
		return '#ef4444';
	}
</script>

<main>
	<h1>Agents</h1>

	{#if agents.length === 0}
		<p class="empty">No agents registered. Use <code>hive add-agent</code> to register one.</p>
	{:else}
		<table>
			<thead>
				<tr>
					<th>Name</th>
					<th>Type</th>
					<th>Health</th>
					<th>Trust</th>
					<th>Capabilities</th>
				</tr>
			</thead>
			<tbody>
				{#each agents as agent}
					<tr>
						<td><strong>{agent.name}</strong></td>
						<td>{agent.type}</td>
						<td>
							<span class="badge" style="background:{statusColor(agent.health_status)}">
								{agent.health_status}
							</span>
						</td>
						<td>{agent.trust_level}</td>
						<td><code>{agent.capabilities}</code></td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</main>

<style>
	main { font-family: system-ui, sans-serif; }
	table { width: 100%; border-collapse: collapse; margin-top: 1rem; }
	th, td { padding: 0.75rem; text-align: left; border-bottom: 1px solid #eee; }
	th { font-weight: 600; color: #666; }
	.badge { color: white; padding: 2px 8px; border-radius: 4px; font-size: 0.8rem; }
	.empty { color: #666; font-style: italic; }
	code { font-size: 0.8rem; background: #f5f5f5; padding: 2px 4px; border-radius: 3px; }
	a { color: #333; }
</style>

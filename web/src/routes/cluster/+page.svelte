<script lang="ts">
	import { fmtRelative } from '$lib/format';
	import { apiGet } from '$lib/api';

	type Member = {
		node_id: string;
		hostname: string;
		address: string;
		status: string;
		last_heartbeat: string;
	};

	let members = $state<Member[]>([]);
	let loading = $state(true);

	async function load() {
		try {
			members = (await apiGet<Member[]>('/api/v1/cluster')) ?? [];
		} catch {
			/* banner shown by apiGet */
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		load();
		const i = setInterval(load, 5000);
		return () => clearInterval(i);
	});

	function statusColor(s: string): string {
		if (s === 'active') return 'var(--ok)';
		if (s === 'draining') return 'var(--warn)';
		return 'var(--err)';
	}
</script>

<main>
	<h1>Cluster</h1>
	<p class="subtitle">Nodes participating in this hive.</p>

	{#if loading}
		<div class="empty">Loading…</div>
	{:else if members.length === 0}
		<div class="empty">Single-node deployment — no roster entries.</div>
	{:else}
		<table>
			<thead>
				<tr><th>Node</th><th>Hostname</th><th>Address</th><th>Status</th><th>Last heartbeat</th></tr>
			</thead>
			<tbody>
				{#each members as m (m.node_id)}
					<tr>
						<td><strong>{m.node_id}</strong></td>
						<td><code>{m.hostname}</code></td>
						<td><code>{m.address}</code></td>
						<td>
							<span class="badge" style="background:{statusColor(m.status)}">{m.status}</span>
						</td>
						<td>{fmtRelative(m.last_heartbeat)}</td>
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

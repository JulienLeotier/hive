<script lang="ts">
	import { fmtRelative } from '$lib/format';
	import { apiGet } from '$lib/api';
	import type { ClusterMember } from '$lib/types';
	import ListScaffold from '$lib/ListScaffold.svelte';

	let members = $state<ClusterMember[]>([]);
	let loading = $state(true);

	async function load() {
		try {
			members = (await apiGet<ClusterMember[]>('/api/v1/cluster')) ?? [];
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

<ListScaffold
	title="Cluster"
	subtitle="Nodes participating in this hive."
	{loading}
	isEmpty={members.length === 0}
	emptyText="Single-node deployment — no roster entries."
>
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
</ListScaffold>

<script lang="ts">
	import { fmtRelative } from '$lib/format';
	import { apiGet } from '$lib/api';
	import type { Workflow } from '$lib/types';
	import ListScaffold from '$lib/ListScaffold.svelte';

	let workflows = $state<Workflow[]>([]);
	let loading = $state(true);

	async function load() {
		try {
			workflows = (await apiGet<Workflow[]>('/api/v1/workflows')) ?? [];
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

	function badgeColor(status: string): string {
		if (status === 'running') return 'var(--warn)';
		if (status === 'completed') return 'var(--ok)';
		if (status === 'failed') return 'var(--err)';
		return 'var(--text-muted)';
	}
</script>

<ListScaffold
	title="Workflows"
	subtitle="Every workflow run recorded on this hive."
	{loading}
	isEmpty={workflows.length === 0}
	emptyText="No workflows yet. Run one with `hive run`."
>
	<table>
		<thead>
			<tr><th>Name</th><th>Status</th><th>ID</th><th>Started</th></tr>
		</thead>
		<tbody>
			{#each workflows as w (w.id)}
				<tr>
					<td><strong>{w.name}</strong></td>
					<td><span class="badge" style="background:{badgeColor(w.status)}">{w.status}</span></td>
					<td><code>{w.id.slice(-12)}</code></td>
					<td>{fmtRelative(w.created_at)}</td>
				</tr>
			{/each}
		</tbody>
	</table>
</ListScaffold>

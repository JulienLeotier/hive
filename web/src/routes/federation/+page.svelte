<script lang="ts">
	import { fmtRelative } from '$lib/format';
	import { apiGet } from '$lib/api';
	import type { FederationLink } from '$lib/types';
	import ListScaffold from '$lib/ListScaffold.svelte';

	let links = $state<FederationLink[]>([]);
	let loading = $state(true);

	async function load() {
		try {
			links = (await apiGet<FederationLink[]>('/api/v1/federation')) ?? [];
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

	function parseCaps(s: string): string[] {
		try {
			return JSON.parse(s ?? '[]');
		} catch {
			return [];
		}
	}
</script>

<ListScaffold
	title="Federation"
	subtitle="Links to peer Hive deployments. Capabilities exchanged, task data stays local."
	{loading}
	isEmpty={links.length === 0}
	emptyText="No federation links. Connect one with `hive federation connect <name> <url>`."
>
	<table>
		<thead>
			<tr><th>Name</th><th>URL</th><th>Status</th><th>Shared capabilities</th><th>Last heartbeat</th></tr>
		</thead>
		<tbody>
			{#each links as l (l.name)}
				<tr>
					<td><strong>{l.name}</strong></td>
					<td><code>{l.url}</code></td>
					<td>
						<span
							class="badge"
							style="background:{l.status === 'active'
								? 'var(--ok)'
								: l.status === 'degraded'
									? 'var(--warn)'
									: 'var(--err)'}">{l.status}</span
						>
					</td>
					<td>
						{#each parseCaps(l.shared_caps) as cap}
							<span class="cap">{cap}</span>
						{/each}
						{#if parseCaps(l.shared_caps).length === 0}
							<span class="muted">all</span>
						{/if}
					</td>
					<td>{l.last_heartbeat ? fmtRelative(l.last_heartbeat) : '—'}</td>
				</tr>
			{/each}
		</tbody>
	</table>
</ListScaffold>

<style>
	.cap {
		display: inline-block;
		padding: 2px 8px;
		border: 1px solid var(--border);
		border-radius: 4px;
		font-size: 0.75rem;
		margin-right: 4px;
	}
	.muted {
		color: var(--text-muted);
		font-style: italic;
	}
</style>

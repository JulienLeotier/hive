<script lang="ts">
	import { apiGet } from '$lib/api';
	import ListScaffold from '$lib/ListScaffold.svelte';

	type CatalogAgent = {
		name: string;
		type: string;
		version?: string;
		task_types: string[];
		cost_per_run?: number;
	};
	type PeerSlice = {
		peer_name: string;
		peer_url: string;
		status: string;
		error?: string;
		agents?: CatalogAgent[];
	};

	let peers = $state<PeerSlice[]>([]);
	let loading = $state(true);
	let search = $state('');

	async function load() {
		try {
			peers = (await apiGet<PeerSlice[]>('/api/v1/marketplace')) ?? [];
		} catch {
			/* banner shown by apiGet */
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		load();
		const i = setInterval(load, 30000);
		return () => clearInterval(i);
	});

	let totalAgents = $derived(
		peers.reduce((acc, p) => acc + (p.agents?.length ?? 0), 0)
	);
	let healthy = $derived(peers.filter((p) => p.status === 'ok').length);

	function matches(a: CatalogAgent, needle: string): boolean {
		if (!needle) return true;
		const n = needle.toLowerCase();
		return (
			a.name.toLowerCase().includes(n) ||
			a.type.toLowerCase().includes(n) ||
			a.task_types.some((t) => t.toLowerCase().includes(n))
		);
	}

	function statusColor(s: string): string {
		if (s === 'ok') return 'var(--ok)';
		if (s === 'unreachable') return 'var(--warn)';
		return 'var(--err)';
	}
</script>

<ListScaffold
	title="Marketplace"
	subtitle="Agents published by every federated peer. Aggregate view — each slice polls the peer's /api/v1/federation/catalog."
	{loading}
	isEmpty={peers.length === 0}
	emptyText="No federated peers. Connect one with `hive federation add`."
>
	{#snippet controls()}
		<div class="controls">
			<input type="text" placeholder="Search agents, types, capabilities…" bind:value={search} />
			<span class="count">{totalAgents} agents · {healthy}/{peers.length} peers healthy</span>
		</div>
	{/snippet}

	{#each peers as p (p.peer_name)}
		<section class="peer">
			<header>
				<div>
					<strong>{p.peer_name}</strong>
					<a class="url" href={p.peer_url} target="_blank" rel="noopener noreferrer">
						{p.peer_url}
					</a>
				</div>
				<span class="badge" style="background:{statusColor(p.status)}">{p.status}</span>
			</header>
			{#if p.error}
				<p class="peer-error">{p.error}</p>
			{:else if !p.agents || p.agents.length === 0}
				<p class="empty-peer">No publishable agents.</p>
			{:else}
				<table>
					<thead>
						<tr>
							<th>Agent</th><th>Type</th><th>Version</th><th>Task types</th><th>Cost/run</th>
						</tr>
					</thead>
					<tbody>
						{#each p.agents.filter((a) => matches(a, search)) as a (a.name)}
							<tr>
								<td><strong>{a.name}</strong></td>
								<td><code>{a.type}</code></td>
								<td><code>{a.version ?? '1.0.0'}</code></td>
								<td>
									{#each a.task_types as tt}
										<span class="chip">{tt}</span>
									{/each}
								</td>
								<td>{a.cost_per_run ? a.cost_per_run.toFixed(3) : '—'}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</section>
	{/each}
</ListScaffold>

<style>
	.controls {
		display: flex;
		gap: 0.5rem;
		align-items: center;
		margin: 1rem 0;
	}
	.controls input {
		flex: 1;
		max-width: 420px;
		padding: 0.4rem 0.7rem;
		border: 1px solid var(--border);
		border-radius: 6px;
		background: var(--bg-panel);
		color: var(--text);
		font: inherit;
	}
	.count {
		color: var(--muted);
		font-size: 0.85rem;
	}
	.peer {
		margin: 1.5rem 0;
		padding: 1rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 6px;
	}
	.peer header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 0.75rem;
	}
	.url {
		margin-left: 0.75rem;
		font-size: 0.8rem;
		color: var(--muted);
		text-decoration: none;
	}
	.url:hover { color: var(--accent); }
	.badge {
		display: inline-block;
		padding: 0.125rem 0.5rem;
		border-radius: 4px;
		color: white;
		font-size: 0.7rem;
		font-weight: 500;
	}
	.peer-error {
		color: var(--err);
		font-family: ui-monospace, monospace;
		font-size: 0.8rem;
		margin: 0;
	}
	.empty-peer {
		color: var(--muted);
		font-style: italic;
		margin: 0;
	}
	.chip {
		display: inline-block;
		padding: 0.1rem 0.4rem;
		margin-right: 0.25rem;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 3px;
		font-size: 0.7rem;
		font-family: ui-monospace, monospace;
	}
</style>

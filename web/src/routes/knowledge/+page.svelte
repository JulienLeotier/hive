<script lang="ts">
	import { fmtRelative, truncate } from '$lib/format';

	type Entry = {
		id: number;
		task_type: string;
		approach: string;
		outcome: string;
		context: string;
		created_at: string;
	};

	let entries = $state<Entry[]>([]);
	let filterType = $state('');
	let searchQuery = $state('');
	let searching = $state(false);
	let loading = $state(true);

	async function load() {
		try {
			const url = filterType ? `/api/v1/knowledge?type=${encodeURIComponent(filterType)}` : '/api/v1/knowledge';
			const r = await fetch(url);
			entries = (await r.json()).data ?? [];
		} catch {
			/* noop */
		}
		loading = false;
	}

	async function search() {
		if (!searchQuery.trim()) {
			await load();
			return;
		}
		searching = true;
		try {
			const r = await fetch(`/api/v1/knowledge/search?q=${encodeURIComponent(searchQuery)}&limit=20`);
			entries = (await r.json()).data ?? [];
		} catch {
			/* noop */
		}
		searching = false;
	}

	$effect(() => {
		load();
	});
</script>

<main>
	<h1>Knowledge</h1>
	<p class="subtitle">Approaches the hive has learned, ranked by similarity + recency.</p>

	<div class="controls">
		<input type="text" placeholder="Task type filter (e.g. code-review)" bind:value={filterType} onkeydown={(e) => e.key === 'Enter' && load()} />
		<button onclick={load}>Filter</button>
		<div class="spacer"></div>
		<input type="text" placeholder="Semantic search (e.g. how to handle timeouts)" bind:value={searchQuery} onkeydown={(e) => e.key === 'Enter' && search()} />
		<button onclick={search}>Search</button>
	</div>

	{#if loading || searching}
		<div class="empty">{searching ? 'Searching…' : 'Loading…'}</div>
	{:else if entries.length === 0}
		<div class="empty">No knowledge entries match.</div>
	{:else}
		<table>
			<thead>
				<tr><th>Type</th><th>Approach</th><th>Outcome</th><th>Age</th></tr>
			</thead>
			<tbody>
				{#each entries as e (e.id)}
					<tr>
						<td><code>{e.task_type}</code></td>
						<td>{truncate(e.approach, 120)}</td>
						<td>
							<span
								class="badge"
								style="background:{e.outcome === 'success' ? 'var(--ok)' : 'var(--err)'}"
								>{e.outcome}</span
							>
						</td>
						<td>{fmtRelative(e.created_at)}</td>
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
	.controls {
		display: flex;
		gap: 0.5rem;
		margin: 1rem 0;
		align-items: center;
	}
	.controls input {
		flex: 1;
		padding: 0.5rem 0.75rem;
		border: 1px solid var(--border);
		border-radius: 6px;
		background: var(--bg-panel);
		color: var(--text);
		font-size: 0.875rem;
	}
	.controls button {
		background: var(--accent);
		color: white;
		border: none;
		padding: 0.5rem 1rem;
		border-radius: 6px;
		cursor: pointer;
	}
	.spacer {
		width: 1rem;
	}
</style>

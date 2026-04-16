<script lang="ts">
	import { fmtRelative } from '$lib/format';

	type Entry = {
		id: number;
		action: string;
		actor: string;
		resource: string;
		detail: string;
		created_at: string;
	};

	let entries = $state<Entry[]>([]);
	let actorFilter = $state('');
	let loading = $state(true);

	async function load() {
		try {
			const r = await fetch('/api/v1/audit');
			entries = (await r.json()).data ?? [];
		} catch {
			/* noop */
		}
		loading = false;
	}

	$effect(() => {
		load();
		const i = setInterval(load, 10000);
		return () => clearInterval(i);
	});

	let filtered = $derived(
		actorFilter ? entries.filter((e) => e.actor.includes(actorFilter)) : entries
	);
</script>

<main>
	<h1>Audit log</h1>
	<p class="subtitle">Every sensitive action: registrations, removals, config changes, trust edits.</p>

	<div class="controls">
		<input type="text" placeholder="Filter by actor" bind:value={actorFilter} />
	</div>

	{#if loading}
		<div class="empty">Loading…</div>
	{:else if filtered.length === 0}
		<div class="empty">No audit entries match.</div>
	{:else}
		<table>
			<thead>
				<tr><th>When</th><th>Actor</th><th>Action</th><th>Resource</th><th>Detail</th></tr>
			</thead>
			<tbody>
				{#each filtered as e (e.id)}
					<tr>
						<td>{fmtRelative(e.created_at)}</td>
						<td><strong>{e.actor}</strong></td>
						<td><code>{e.action}</code></td>
						<td>{e.resource}</td>
						<td class="detail">{e.detail}</td>
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
		margin: 1rem 0;
	}
	.controls input {
		padding: 0.5rem 0.75rem;
		border: 1px solid var(--border);
		border-radius: 6px;
		background: var(--bg-panel);
		color: var(--text);
		width: 300px;
	}
	.detail {
		color: var(--text-muted);
		font-size: 0.85rem;
		max-width: 500px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
</style>

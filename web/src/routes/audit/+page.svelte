<script lang="ts">
	import { fmtRelative } from '$lib/format';
	import { apiGet } from '$lib/api';
	import type { AuditEntry } from '$lib/types';
	import ListScaffold from '$lib/ListScaffold.svelte';

	let entries = $state<AuditEntry[]>([]);
	let actorFilter = $state('');
	let actionFilter = $state('');
	let resourceFilter = $state('');
	let search = $state('');
	let sinceHours = $state(24);
	let loading = $state(true);

	async function load() {
		try {
			entries = (await apiGet<AuditEntry[]>('/api/v1/audit')) ?? [];
		} catch {
			/* banner shown by apiGet */
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		load();
		const i = setInterval(load, 10000);
		return () => clearInterval(i);
	});

	// Multi-field filter: every active field narrows the set (AND). Free-text
	// `search` matches the detail payload case-insensitively so operators can
	// grep for task IDs or IP addresses without picking a specific column.
	let filtered = $derived(() => {
		const cutoff = sinceHours > 0 ? Date.now() - sinceHours * 3600 * 1000 : 0;
		const needle = search.trim().toLowerCase();
		return entries.filter((e) => {
			if (actorFilter && !e.actor.toLowerCase().includes(actorFilter.toLowerCase())) return false;
			if (actionFilter && !e.action.toLowerCase().includes(actionFilter.toLowerCase())) return false;
			if (resourceFilter && !e.resource.toLowerCase().includes(resourceFilter.toLowerCase())) return false;
			if (cutoff > 0) {
				const t = new Date(e.created_at).getTime();
				if (!isNaN(t) && t < cutoff) return false;
			}
			if (needle) {
				const hay = [e.actor, e.action, e.resource, e.detail ?? '']
					.join(' ')
					.toLowerCase();
				if (!hay.includes(needle)) return false;
			}
			return true;
		});
	});

	function clearFilters() {
		actorFilter = '';
		actionFilter = '';
		resourceFilter = '';
		search = '';
		sinceHours = 24;
	}
</script>

<ListScaffold
	title="Audit log"
	subtitle="Every sensitive action: registrations, removals, config changes, trust edits."
	{loading}
	isEmpty={filtered().length === 0}
	emptyText="No audit entries match these filters."
>
	{#snippet controls()}
		<div class="controls">
			<input type="text" placeholder="Actor" bind:value={actorFilter} />
			<input type="text" placeholder="Action" bind:value={actionFilter} />
			<input type="text" placeholder="Resource" bind:value={resourceFilter} />
			<select bind:value={sinceHours} title="Time window">
				<option value={1}>last hour</option>
				<option value={24}>last 24h</option>
				<option value={168}>last 7 days</option>
				<option value={720}>last 30 days</option>
				<option value={0}>all time</option>
			</select>
			<input type="text" class="search" placeholder="Search detail…" bind:value={search} />
			{#if actorFilter || actionFilter || resourceFilter || search || sinceHours !== 24}
				<button class="clear" onclick={clearFilters}>Clear</button>
			{/if}
			<span class="count">{filtered().length} / {entries.length}</span>
		</div>
	{/snippet}

	<table>
		<thead>
			<tr><th>When</th><th>Actor</th><th>Action</th><th>Resource</th><th>Detail</th></tr>
		</thead>
		<tbody>
			{#each filtered() as e (e.id)}
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
</ListScaffold>

<style>
	.controls {
		margin: 1rem 0;
		display: flex;
		gap: 0.4rem;
		flex-wrap: wrap;
		align-items: center;
	}
	.controls input,
	.controls select {
		padding: 0.4rem 0.6rem;
		border: 1px solid var(--border);
		border-radius: 6px;
		background: var(--bg-panel);
		color: var(--text);
		font: inherit;
	}
	.controls input { width: 140px; }
	.controls .search { width: 240px; flex: 1; min-width: 200px; }
	.controls .clear {
		padding: 0.4rem 0.7rem;
		background: transparent;
		border: 1px solid var(--border);
		border-radius: 6px;
		color: var(--muted);
		cursor: pointer;
		font: inherit;
	}
	.controls .clear:hover {
		color: var(--err);
		border-color: var(--err);
	}
	.controls .count {
		margin-left: auto;
		color: var(--muted);
		font-size: 0.8rem;
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

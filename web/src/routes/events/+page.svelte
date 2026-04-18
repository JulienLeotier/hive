<script lang="ts">
	import { apiGet } from '$lib/api';
	import { createReconnectingWS, wsURL } from '$lib/ws';
	import { fmtRelative } from '$lib/format';
	import type { Event } from '$lib/types';

	let events = $state<Event[]>([]);
	let loading = $state(true);
	let typeFilter = $state('');
	let sourceFilter = $state('');
	let expanded = $state<Record<number, boolean>>({});

	async function loadEvents() {
		loading = true;
		try {
			const params = new URLSearchParams();
			if (typeFilter) params.set('type', typeFilter);
			if (sourceFilter) params.set('source', sourceFilter);
			const qs = params.toString();
			const url = qs ? `/api/v1/events?${qs}` : '/api/v1/events';
			events = (await apiGet<Event[]>(url)) ?? [];
		} catch {
			/* banner shown by apiGet */
		} finally {
			loading = false;
		}
	}

	function matchesFilters(evt: Event): boolean {
		if (typeFilter && !evt.type.startsWith(typeFilter)) return false;
		if (sourceFilter && evt.source !== sourceFilter) return false;
		return true;
	}

	function clearFilters() {
		typeFilter = '';
		sourceFilter = '';
		loadEvents();
	}

	$effect(() => {
		loadEvents();
		const ws = createReconnectingWS({
			url: wsURL('/ws'),
			onmessage: (msg) => {
				try {
					const evt = JSON.parse(msg.data) as Event;
					if (!matchesFilters(evt)) return;
					events = [evt, ...events].slice(0, 100);
				} catch {
					/* ignore non-JSON frames */
				}
			}
		});
		return () => ws.close();
	});

	function eventColor(type: string): string {
		if (type === 'project.shipped' || type === 'story.reviewed' || type.endsWith('_done')) return 'var(--ok)';
		if (type.endsWith('.failed') || type === 'story.blocked' || type.includes('cap_reached')) return 'var(--err)';
		if (type.includes('warning')) return 'var(--warn)';
		return 'var(--accent)';
	}

	function eventIcon(type: string): string {
		if (type === 'project.shipped') return '🚀';
		if (type.endsWith('.failed')) return '✕';
		if (type.includes('warning')) return '⚠';
		if (type === 'story.reviewed') return '✓';
		if (type.startsWith('project.bmad_step')) return '●';
		if (type.startsWith('story.dev')) return '◆';
		if (type.startsWith('project.architect')) return '◎';
		if (type.startsWith('story.pr')) return '⬆';
		return '·';
	}

	function prettyPayload(raw: string): string {
		try {
			return JSON.stringify(JSON.parse(raw), null, 2);
		} catch {
			return raw;
		}
	}

	function toggleExpand(id: number) {
		expanded = { ...expanded, [id]: !expanded[id] };
	}
</script>

<svelte:head><title>Événements · Hive</title></svelte:head>

<h1>Événements</h1>
<p class="sub">Flux live des events émis par le bus Hive. Tap un event pour voir son payload complet.</p>

<div class="filters">
	<div class="filter-row">
		<input bind:value={typeFilter}
			placeholder="Filtrer par type (préfixe)"
			onkeydown={(e) => e.key === 'Enter' && loadEvents()} />
		<input bind:value={sourceFilter}
			placeholder="Filtrer par source"
			onkeydown={(e) => e.key === 'Enter' && loadEvents()} />
	</div>
	<div class="filter-actions">
		<button class="btn primary" onclick={loadEvents}>Appliquer</button>
		{#if typeFilter || sourceFilter}
			<button class="btn ghost" onclick={clearFilters}>Effacer</button>
		{/if}
		<span class="count">
			{events.length} event{events.length > 1 ? 's' : ''}
		</span>
	</div>
</div>

{#if loading && events.length === 0}
	<div class="empty"><span class="empty-icon">⏳</span>Chargement…</div>
{:else if events.length === 0}
	<div class="empty">
		<span class="empty-icon">◌</span>
		{typeFilter || sourceFilter ? 'Aucun event ne matche ce filtre.' : 'Silence radio. Lance un projet pour voir des events apparaître ici.'}
	</div>
{:else}
	<ul class="timeline">
		{#each events as evt (evt.id)}
			<li class="evt" class:expanded={expanded[evt.id]}>
				<button class="evt-head" onclick={() => toggleExpand(evt.id)}>
					<span class="evt-icon" style="color:{eventColor(evt.type)}">{eventIcon(evt.type)}</span>
					<span class="evt-type" style="color:{eventColor(evt.type)}">{evt.type}</span>
					<span class="evt-source">{evt.source}</span>
					<span class="evt-time">{fmtRelative(evt.created_at)}</span>
				</button>
				{#if expanded[evt.id]}
					<pre class="evt-payload">{prettyPayload(evt.payload)}</pre>
				{/if}
			</li>
		{/each}
	</ul>
{/if}

<style>
	.sub {
		color: var(--text-muted);
		font-size: 0.85rem;
		margin: 0 0 1.25rem;
	}

	/* ===== Filters ===== */
	.filters {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
		margin-bottom: 1.25rem;
	}
	.filter-row {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 0.5rem;
	}
	.filter-row input {
		padding: 0.55rem 0.75rem;
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 6px;
		color: inherit;
		font: inherit;
		font-size: 0.85rem;
	}
	.filter-row input:focus {
		outline: none;
		border-color: var(--accent);
	}
	.filter-actions {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		flex-wrap: wrap;
	}
	.btn {
		padding: 0.5rem 0.95rem;
		border-radius: 6px;
		font-size: 0.85rem;
		font-weight: 600;
		cursor: pointer;
		border: 1px solid var(--border);
		background: var(--bg-panel);
		color: inherit;
	}
	.btn.primary {
		background: var(--accent);
		color: white;
		border-color: var(--accent);
	}
	.btn.primary:hover { background: color-mix(in srgb, var(--accent) 88%, black); }
	.btn.ghost:hover { border-color: var(--accent); color: var(--accent); }
	.count {
		margin-left: auto;
		font-size: 0.8rem;
		color: var(--text-muted);
		font-variant-numeric: tabular-nums;
	}

	/* ===== Timeline ===== */
	.timeline {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 0.35rem;
	}
	.evt {
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 6px;
		overflow: hidden;
		transition: border-color 0.1s;
	}
	.evt:hover { border-color: var(--accent); }
	.evt.expanded { border-color: var(--accent); }
	.evt-head {
		display: flex;
		align-items: center;
		gap: 0.7rem;
		width: 100%;
		padding: 0.6rem 0.85rem;
		background: transparent;
		border: none;
		color: inherit;
		font: inherit;
		cursor: pointer;
		text-align: left;
	}
	.evt-icon {
		font-size: 0.95rem;
		font-family: ui-monospace, monospace;
		width: 18px;
		text-align: center;
		flex-shrink: 0;
		line-height: 1;
	}
	.evt-type {
		font-family: ui-monospace, monospace;
		font-size: 0.82rem;
		font-weight: 600;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		flex: 1;
		min-width: 0;
	}
	.evt-source {
		font-size: 0.75rem;
		color: var(--text-muted);
		background: var(--bg-hover);
		padding: 2px 8px;
		border-radius: 10px;
		flex-shrink: 0;
	}
	.evt-time {
		font-size: 0.72rem;
		color: var(--text-muted);
		white-space: nowrap;
		flex-shrink: 0;
		font-variant-numeric: tabular-nums;
	}
	.evt-payload {
		margin: 0;
		padding: 0.7rem 0.85rem;
		background: var(--bg);
		color: var(--text);
		font-family: ui-monospace, monospace;
		font-size: 0.75rem;
		line-height: 1.5;
		white-space: pre-wrap;
		word-break: break-word;
		max-height: 320px;
		overflow-y: auto;
		border-top: 1px dashed var(--border);
	}

	/* ===== Empty ===== */
	.empty {
		padding: 3rem 1rem;
		text-align: center;
		color: var(--text-muted);
		background: var(--bg-panel);
		border: 1px dashed var(--border);
		border-radius: 8px;
	}
	.empty-icon {
		display: block;
		font-size: 2rem;
		margin-bottom: 0.5rem;
		opacity: 0.5;
	}

	/* ===== Responsive ===== */
	@media (max-width: 767px) {
		.filter-row { grid-template-columns: 1fr; }
		.evt-head { flex-wrap: wrap; gap: 0.4rem 0.7rem; }
		.evt-type { flex: 1 1 100%; order: 2; }
		.evt-source { order: 3; }
		.evt-time { order: 4; margin-left: auto; }
		.evt-icon { order: 1; }
	}
</style>

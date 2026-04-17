<script lang="ts">
	import { apiGet } from '$lib/api';
	import { createReconnectingWS, wsURL } from '$lib/ws';
	import type { Event } from '$lib/types';

	let events = $state<Event[]>([]);
	let typeFilter = $state('');
	let sourceFilter = $state('');

	async function loadEvents() {
		try {
			const params = new URLSearchParams();
			if (typeFilter) params.set('type', typeFilter);
			if (sourceFilter) params.set('source', sourceFilter);
			const qs = params.toString();
			const url = qs ? `/api/v1/events?${qs}` : '/api/v1/events';
			events = (await apiGet<Event[]>(url)) ?? [];
		} catch {
			/* banner shown by apiGet */
		}
	}

	// Client-side mirror of the active filters for incoming WS events. Without
	// this, a filtered view would get polluted by unrelated events pushed over
	// the socket.
	function matchesFilters(evt: Event): boolean {
		if (typeFilter && !evt.type.startsWith(typeFilter)) return false;
		if (sourceFilter && evt.source !== sourceFilter) return false;
		return true;
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
</script>

<main>
	<h1>Event Timeline</h1>

	<div class="filter">
		<input bind:value={typeFilter} placeholder="Filter by type prefix (e.g., task)" />
		<input bind:value={sourceFilter} placeholder="Filter by source" />
		<button onclick={loadEvents}>Filter</button>
	</div>

	{#if events.length === 0}
		<p class="empty">No events yet.</p>
	{:else}
		<div class="timeline">
			{#each events as evt}
				<div class="event">
					<span class="time">{evt.created_at}</span>
					<span class="type">{evt.type}</span>
					<span class="source">{evt.source}</span>
					<code class="payload">{evt.payload}</code>
				</div>
			{/each}
		</div>
	{/if}
</main>

<style>
	main { font-family: system-ui, sans-serif; }
	.filter { display: flex; gap: 0.5rem; margin: 1rem 0; }
	.filter input { flex: 1; padding: 0.5rem; border: 1px solid #ddd; border-radius: 4px; }
	.filter button { padding: 0.5rem 1rem; background: #333; color: white; border: none; border-radius: 4px; cursor: pointer; }
	.timeline { display: flex; flex-direction: column; gap: 0.5rem; }
	.event { display: flex; gap: 1rem; align-items: center; padding: 0.5rem; border-left: 3px solid #3b82f6; background: #fafafa; border-radius: 0 4px 4px 0; }
	.time { font-size: 0.75rem; color: #666; min-width: 80px; }
	.type { font-weight: 600; min-width: 150px; }
	.source { color: #666; min-width: 100px; }
	.payload { font-size: 0.7rem; color: #888; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 300px; }
	.empty { color: #666; font-style: italic; }
	a { color: #333; }
</style>

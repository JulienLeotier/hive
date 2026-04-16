<script lang="ts">
	type Event = { id: number; type: string; source: string; payload: string; created_at: string; };

	let events = $state<Event[]>([]);
	let typeFilter = $state('');
	let ws: WebSocket | null = $state(null);

	async function loadEvents() {
		try {
			const url = typeFilter ? `/api/v1/events?type=${typeFilter}` : '/api/v1/events';
			const res = await fetch(url);
			const json = await res.json();
			events = json.data ?? [];
		} catch { /* API not ready */ }
	}

	function connectWS() {
		const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
		ws = new WebSocket(`${proto}//${location.host}/ws`);
		ws.onmessage = (msg) => {
			const evt = JSON.parse(msg.data);
			events = [evt, ...events].slice(0, 100);
		};
		ws.onclose = () => setTimeout(connectWS, 3000);
	}

	$effect(() => {
		loadEvents();
		connectWS();
		return () => ws?.close();
	});
</script>

<main>
	<h1>Event Timeline</h1>

	<div class="filter">
		<input bind:value={typeFilter} placeholder="Filter by type (e.g., task)" />
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

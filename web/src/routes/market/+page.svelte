<script lang="ts">
	import { fmtRelative, fmtUSD } from '$lib/format';

	type Auction = {
		id: string;
		task_id: string;
		strategy: string;
		status: string;
		winner: string;
		bids: number;
		opened_at: string;
	};

	let auctions = $state<Auction[]>([]);
	let loading = $state(true);

	async function load() {
		try {
			const r = await fetch('/api/v1/auctions');
			auctions = (await r.json()).data ?? [];
		} catch {
			/* noop */
		}
		loading = false;
	}

	$effect(() => {
		load();
		const i = setInterval(load, 5000);
		return () => clearInterval(i);
	});
</script>

<main>
	<h1>Market</h1>
	<p class="subtitle">Agent auctions for task allocation.</p>

	{#if loading}
		<div class="empty">Loading…</div>
	{:else if auctions.length === 0}
		<div class="empty">
			No auctions. Configure a workflow with <code>allocation: market</code> or open one manually with <code>hive auction open</code>.
		</div>
	{:else}
		<table>
			<thead>
				<tr><th>Auction</th><th>Task</th><th>Strategy</th><th>Bids</th><th>Winner</th><th>Status</th><th>Opened</th></tr>
			</thead>
			<tbody>
				{#each auctions as a (a.id)}
					<tr>
						<td><code>{a.id.slice(-8)}</code></td>
						<td><code>{a.task_id.slice(-8)}</code></td>
						<td>{a.strategy}</td>
						<td>{a.bids}</td>
						<td>{a.winner || '—'}</td>
						<td>
							<span
								class="badge"
								style="background:{a.status === 'closed'
									? 'var(--ok)'
									: a.status === 'cancelled'
										? 'var(--err)'
										: 'var(--warn)'}">{a.status}</span
							>
						</td>
						<td>{fmtRelative(a.opened_at)}</td>
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
</style>

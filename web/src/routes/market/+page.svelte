<script lang="ts">
	import { fmtRelative } from '$lib/format';
	import { apiGet } from '$lib/api';
	import type { Auction } from '$lib/types';
	import ListScaffold from '$lib/ListScaffold.svelte';

	let auctions = $state<Auction[]>([]);
	let loading = $state(true);

	async function load() {
		try {
			auctions = (await apiGet<Auction[]>('/api/v1/auctions')) ?? [];
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
</script>

<ListScaffold
	title="Market"
	subtitle="Agent auctions for task allocation."
	{loading}
	isEmpty={auctions.length === 0}
	emptyText="No auctions. Configure a workflow with `allocation: market` or open one manually with `hive auction open`."
>
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
</ListScaffold>

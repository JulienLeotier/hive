<script lang="ts">
	type Summary = { agent_name: string; total_cost: number; task_count: number };
	type Alert = { agent_name: string; daily_limit: number; spend: number; breached: boolean };

	let summaries = $state<Summary[]>([]);
	let alerts = $state<Alert[]>([]);
	let loading = $state(true);

	async function load() {
		try {
			const res = await fetch('/api/v1/costs');
			const json = await res.json();
			summaries = json.data?.summaries ?? [];
			alerts = json.data?.alerts ?? [];
		} catch {
			/* API not ready */
		}
		loading = false;
	}

	$effect(() => {
		load();
		const interval = setInterval(load, 5000);
		return () => clearInterval(interval);
	});

	let totalSpend = $derived(summaries.reduce((sum, s) => sum + s.total_cost, 0));
	let breaches = $derived(alerts.filter((a) => a.breached));

	function fmt(n: number): string {
		return `$${n.toFixed(4)}`;
	}
	function pct(spend: number, limit: number): number {
		return limit === 0 ? 0 : Math.min(100, (spend / limit) * 100);
	}
</script>

<main>
	<h1>Costs</h1>

	{#if loading}
		<p class="empty">Loading…</p>
	{:else}
		<div class="stats">
			<div class="stat">
				<span class="label">Total spend</span>
				<span class="value">{fmt(totalSpend)}</span>
			</div>
			<div class="stat">
				<span class="label">Agents tracked</span>
				<span class="value">{summaries.length}</span>
			</div>
			<div class="stat" class:warn={breaches.length > 0}>
				<span class="label">Budget breaches</span>
				<span class="value">{breaches.length}</span>
			</div>
		</div>

		{#if alerts.length > 0}
			<section>
				<h2>Budgets</h2>
				<table>
					<thead>
						<tr>
							<th>Agent</th>
							<th>Daily limit</th>
							<th>Today's spend</th>
							<th>Utilisation</th>
							<th>Status</th>
						</tr>
					</thead>
					<tbody>
						{#each alerts as a (a.agent_name)}
							<tr class:breached={a.breached}>
								<td>{a.agent_name}</td>
								<td>{fmt(a.daily_limit)}</td>
								<td>{fmt(a.spend)}</td>
								<td>
									<div class="bar">
										<div class="fill" style="width:{pct(a.spend, a.daily_limit)}%"></div>
									</div>
								</td>
								<td>
									<span class="badge" class:breached={a.breached}>
										{a.breached ? 'BREACHED' : 'ok'}
									</span>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</section>
		{/if}

		<section>
			<h2>Spend by agent (all-time)</h2>
			{#if summaries.length === 0}
				<p class="empty">No cost entries yet.</p>
			{:else}
				<table>
					<thead>
						<tr>
							<th>Agent</th>
							<th>Total cost</th>
							<th>Tasks</th>
							<th>Avg / task</th>
						</tr>
					</thead>
					<tbody>
						{#each summaries as s (s.agent_name)}
							<tr>
								<td>{s.agent_name}</td>
								<td>{fmt(s.total_cost)}</td>
								<td>{s.task_count}</td>
								<td>{fmt(s.task_count === 0 ? 0 : s.total_cost / s.task_count)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</section>
	{/if}
</main>

<style>
	main {
		font-family: system-ui, sans-serif;
	}
	.stats {
		display: flex;
		gap: 1rem;
		margin: 1rem 0 2rem;
	}
	.stat {
		flex: 1;
		padding: 1rem;
		border: 1px solid #e5e7eb;
		border-radius: 8px;
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}
	.stat.warn {
		border-color: #f87171;
		background: #fef2f2;
	}
	.label {
		font-size: 0.75rem;
		text-transform: uppercase;
		color: #64748b;
	}
	.value {
		font-size: 1.5rem;
		font-weight: 600;
	}
	section {
		margin: 2rem 0;
	}
	section h2 {
		font-size: 1rem;
		margin-bottom: 0.5rem;
	}
	table {
		width: 100%;
		border-collapse: collapse;
	}
	th,
	td {
		padding: 0.5rem 0.75rem;
		text-align: left;
		border-bottom: 1px solid #f1f5f9;
	}
	th {
		font-weight: 600;
		color: #64748b;
		background: #fafafa;
		font-size: 0.75rem;
		text-transform: uppercase;
	}
	tr.breached {
		background: #fef2f2;
	}
	.bar {
		width: 100%;
		height: 8px;
		background: #e5e7eb;
		border-radius: 4px;
		overflow: hidden;
	}
	.fill {
		height: 100%;
		background: linear-gradient(to right, #22c55e, #f59e0b, #ef4444);
	}
	.badge {
		display: inline-block;
		padding: 0.125rem 0.5rem;
		border-radius: 4px;
		background: #22c55e;
		color: white;
		font-size: 0.75rem;
		font-weight: 500;
	}
	.badge.breached {
		background: #ef4444;
	}
	.empty {
		color: #666;
		font-style: italic;
	}
</style>

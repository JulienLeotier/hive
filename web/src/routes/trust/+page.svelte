<script lang="ts">
	import { fmtRelative } from '$lib/format';
	import { apiGet } from '$lib/api';

	type Agent = { id: string; name: string; trust_level: string; health_status: string };
	type Promotion = {
		id: string;
		agent: string;
		old_level: string;
		new_level: string;
		reason: string;
		criteria: string;
		created_at: string;
	};

	let agents = $state<Agent[]>([]);
	let history = $state<Promotion[]>([]);
	let loading = $state(true);

	async function load() {
		try {
			const [a, h] = await Promise.all([
				apiGet<Agent[]>('/api/v1/agents'),
				apiGet<Promotion[]>('/api/v1/trust')
			]);
			agents = a ?? [];
			history = h ?? [];
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

	const levels = ['scripted', 'supervised', 'guided', 'autonomous', 'trusted'];
	function levelIndex(l: string): number {
		const i = levels.indexOf(l);
		return i < 0 ? 0 : i;
	}
	function levelColor(l: string): string {
		const palette = ['#94a3b8', '#64748b', '#3b82f6', '#8b5cf6', '#f59e0b'];
		return palette[levelIndex(l)] ?? '#94a3b8';
	}
</script>

<main>
	<h1>Trust</h1>
	<p class="subtitle">Per-agent autonomy level and promotion history.</p>

	<h2>Current levels</h2>
	{#if loading}
		<div class="empty">Loading…</div>
	{:else if agents.length === 0}
		<div class="empty">No agents registered.</div>
	{:else}
		<div class="trust-grid">
			{#each agents as a (a.id)}
				<div class="trust-card">
					<div class="name">{a.name}</div>
					<div class="level-bar">
						{#each levels as l, i}
							<div
								class="step"
								class:active={i <= levelIndex(a.trust_level)}
								style="--c:{levelColor(a.trust_level)}"
								title={l}
							></div>
						{/each}
					</div>
					<div class="level-text" style="color:{levelColor(a.trust_level)}">
						{a.trust_level}
					</div>
				</div>
			{/each}
		</div>
	{/if}

	<h2>Promotion history</h2>
	{#if history.length === 0}
		<div class="empty">No promotions yet.</div>
	{:else}
		<table>
			<thead>
				<tr><th>Agent</th><th>From</th><th>To</th><th>Reason</th><th>Criteria</th><th>When</th></tr>
			</thead>
			<tbody>
				{#each history as h (h.id)}
					<tr>
						<td><strong>{h.agent}</strong></td>
						<td>{h.old_level}</td>
						<td>
							<span class="badge" style="background:{levelColor(h.new_level)}">{h.new_level}</span>
						</td>
						<td>{h.reason}</td>
						<td><code>{h.criteria}</code></td>
						<td>{fmtRelative(h.created_at)}</td>
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
	.trust-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
		gap: 0.75rem;
		margin: 1rem 0 2rem;
	}
	.trust-card {
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 0.75rem 1rem;
	}
	.name {
		font-weight: 600;
		margin-bottom: 0.5rem;
	}
	.level-bar {
		display: flex;
		gap: 2px;
		margin: 0.5rem 0;
	}
	.step {
		flex: 1;
		height: 6px;
		background: var(--border);
		border-radius: 2px;
	}
	.step.active {
		background: var(--c);
	}
	.level-text {
		font-size: 0.875rem;
		text-transform: capitalize;
	}
</style>

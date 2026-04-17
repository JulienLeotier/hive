<script lang="ts">
	import { fmtRelative } from '$lib/format';
	import { apiGet } from '$lib/api';
	import type { Recommendation, AppliedOptimization } from '$lib/types';

	let recommendations = $state<Recommendation[]>([]);
	let applied = $state<AppliedOptimization[]>([]);
	let loading = $state(true);

	async function load() {
		try {
			const [r1, r2] = await Promise.all([
				apiGet<Recommendation[]>('/api/v1/recommendations'),
				apiGet<AppliedOptimization[]>('/api/v1/optimizations')
			]);
			recommendations = r1 ?? [];
			applied = r2 ?? [];
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

	function typeColor(t: string): string {
		if (t === 'comparative-slowdown') return 'var(--err)';
		if (t === 'slow-agent') return 'var(--warn)';
		if (t === 'idle-agent') return 'var(--text-muted)';
		if (t === 'parallelize') return 'var(--accent)';
		return 'var(--text-muted)';
	}
</script>

<main>
	<h1>Optimizer</h1>
	<p class="subtitle">Recommendations from historical execution + the record of applied tunings.</p>

	<h2>Live recommendations</h2>
	{#if loading}
		<div class="empty">Loading…</div>
	{:else if recommendations.length === 0}
		<div class="empty">No optimization opportunities detected right now.</div>
	{:else}
		<div class="recs">
			{#each recommendations as r}
				<div class="rec" style="border-left: 3px solid {typeColor(r.type)}">
					<div class="rec-head">
						<span class="type">{r.type}</span>
						<span class="confidence">{Math.round(r.confidence * 100)}% confidence</span>
					</div>
					<div class="rec-desc">{r.description}</div>
					<div class="rec-impact">{r.impact}</div>
				</div>
			{/each}
		</div>
	{/if}

	<h2>Applied tunings</h2>
	{#if applied.length === 0}
		<div class="empty">No tunings applied yet. Run <code>hive optimize --apply</code>.</div>
	{:else}
		<table>
			<thead>
				<tr><th>Setting</th><th>Old → New</th><th>Rationale</th><th>When</th></tr>
			</thead>
			<tbody>
				{#each applied as a (a.id)}
					<tr>
						<td><code>{a.setting}</code></td>
						<td>{a.old_value.toFixed(2)} → <strong>{a.new_value.toFixed(2)}</strong></td>
						<td>{a.rationale}</td>
						<td>{fmtRelative(a.applied_at)}</td>
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
	.recs {
		display: grid;
		gap: 0.5rem;
		margin: 1rem 0;
	}
	.rec {
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 0 8px 8px 0;
		padding: 0.75rem 1rem;
	}
	.rec-head {
		display: flex;
		justify-content: space-between;
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--text-muted);
		margin-bottom: 0.25rem;
	}
	.rec-desc {
		margin-bottom: 0.25rem;
	}
	.rec-impact {
		color: var(--text-muted);
		font-size: 0.85rem;
		font-style: italic;
	}
</style>

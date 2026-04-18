<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { apiGet } from '$lib/api';

	type ProjectLine = {
		id: string;
		name: string;
		status: string;
		total_usd: number;
		cap_usd?: number;
		step_count: number;
		failure_stage?: string;
	};
	type PhaseLine = {
		phase: string;
		total_usd: number;
		step_count: number;
		input_tokens: number;
		output_tokens: number;
	};
	type CommandLine = {
		command: string;
		total_usd: number;
		step_count: number;
	};
	type Summary = {
		grand_total_usd: number;
		projects: ProjectLine[];
		phases: PhaseLine[];
		commands: CommandLine[];
	};

	let summary = $state<Summary | null>(null);
	let loading = $state(true);
	let refreshHandle: ReturnType<typeof setInterval> | null = null;

	// Pour la projection : on compare le grand_total actuel à celui d'il y
	// a 60s (un tick de rafraîchissement précédent) et on extrapole à
	// l'heure. Zero-alloc : juste deux floats en mémoire.
	let lastTotalUSD = $state<number | null>(null);
	let lastSampledAt = $state<number | null>(null);
	let currentRateUSDPerHour = $state(0);

	async function load() {
		try {
			const next = await apiGet<Summary>('/api/v1/costs');
			if (next && lastTotalUSD !== null && lastSampledAt !== null) {
				const deltaUSD = next.grand_total_usd - lastTotalUSD;
				const deltaHours = (Date.now() - lastSampledAt) / 3_600_000;
				if (deltaHours > 0 && deltaUSD >= 0) {
					currentRateUSDPerHour = deltaUSD / deltaHours;
				}
			}
			if (next) {
				lastTotalUSD = next.grand_total_usd;
				lastSampledAt = Date.now();
			}
			summary = next;
		} catch {
			/* banner */
		} finally {
			loading = false;
		}
	}

	function fmt(n: number): string {
		if (!Number.isFinite(n)) return '—';
		if (n === 0) return '$0.00';
		if (n < 0.01) return `$${n.toFixed(4)}`;
		return `$${n.toFixed(2)}`;
	}

	function fmtTokens(n: number): string {
		if (!Number.isFinite(n) || n === 0) return '0';
		if (n < 1000) return String(n);
		if (n < 1_000_000) return `${(n / 1000).toFixed(1)}k`;
		return `${(n / 1_000_000).toFixed(2)}M`;
	}

	function barWidth(v: number, max: number): string {
		if (max <= 0) return '0%';
		return `${Math.min(100, (v / max) * 100)}%`;
	}

	onMount(() => {
		load();
		// Refresh every 15s — the cost ticker updates as stories run.
		refreshHandle = setInterval(load, 15_000);
	});

	onDestroy(() => {
		if (refreshHandle) clearInterval(refreshHandle);
	});

	let maxProjectUSD = $derived(
		summary?.projects?.reduce((m, p) => Math.max(m, p.total_usd), 0) ?? 0
	);
	let maxPhaseUSD = $derived(
		summary?.phases?.reduce((m, p) => Math.max(m, p.total_usd), 0) ?? 0
	);
	let maxCmdUSD = $derived(
		summary?.commands?.reduce((m, c) => Math.max(m, c.total_usd), 0) ?? 0
	);
</script>

<svelte:head><title>Coûts · Hive</title></svelte:head>

<h1>Coûts Claude</h1>
<p class="sub">
	Cumul des tokens consommés par tous les projets BMAD. Rafraîchissement auto toutes les 15s.
</p>

{#if loading}
	<div class="empty">Chargement…</div>
{:else if !summary}
	<div class="empty">Aucune donnée.</div>
{:else}
	<div class="summary-card">
		<div class="summary-row">
			<div>
				<div class="big">{fmt(summary.grand_total_usd)}</div>
				<div class="muted">total dépensé · {summary.projects.length} projet(s)</div>
			</div>
			{#if currentRateUSDPerHour > 0}
				<div class="rate">
					<div class="rate-big">{fmt(currentRateUSDPerHour)}<span class="unit">/h</span></div>
					<div class="muted">rythme actuel · ≈ {fmt(currentRateUSDPerHour * 24)} / jour</div>
				</div>
			{/if}
			<a class="csv-btn" href="/api/v1/costs.csv" download>↓ Export CSV</a>
		</div>
	</div>

	<h2>Par projet</h2>
	{#if summary.projects.length === 0}
		<div class="empty">Aucun projet.</div>
	{:else}
		<div class="table-scroll">
		<table>
			<thead>
				<tr>
					<th>Projet</th>
					<th>Statut</th>
					<th class="num">Coût</th>
					<th class="num">Plafond</th>
					<th class="num">Steps</th>
					<th>Consommation</th>
				</tr>
			</thead>
			<tbody>
				{#each summary.projects as p (p.id)}
					<tr>
						<td>
							<a href={`/projects/${encodeURIComponent(p.id)}`}>{p.name || p.id}</a>
							{#if p.failure_stage}
								<span class="fail-tag" title={p.failure_stage}>failed</span>
							{/if}
						</td>
						<td><span class="status s-{p.status}">{p.status}</span></td>
						<td class="num mono">{fmt(p.total_usd)}</td>
						<td class="num mono">{p.cap_usd ? fmt(p.cap_usd) : '—'}</td>
						<td class="num mono">{p.step_count}</td>
						<td>
							<div class="bar">
								<div class="bar-fill"
									class:over-cap={p.cap_usd && p.total_usd >= p.cap_usd}
									style:width={barWidth(p.total_usd, maxProjectUSD)}>
								</div>
							</div>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
		</div>
	{/if}

	<h2>Par phase BMAD</h2>
	{#if summary.phases.length === 0}
		<div class="empty">Aucune phase enregistrée.</div>
	{:else}
		<div class="table-scroll">
		<table>
			<thead>
				<tr>
					<th>Phase</th>
					<th class="num">Coût</th>
					<th class="num">Steps</th>
					<th class="num">Tokens in</th>
					<th class="num">Tokens out</th>
					<th>Consommation</th>
				</tr>
			</thead>
			<tbody>
				{#each summary.phases as p}
					<tr>
						<td>{p.phase}</td>
						<td class="num mono">{fmt(p.total_usd)}</td>
						<td class="num mono">{p.step_count}</td>
						<td class="num mono">{fmtTokens(p.input_tokens)}</td>
						<td class="num mono">{fmtTokens(p.output_tokens)}</td>
						<td>
							<div class="bar">
								<div class="bar-fill" style:width={barWidth(p.total_usd, maxPhaseUSD)}></div>
							</div>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
		</div>
	{/if}

	<h2>Top commandes BMAD</h2>
	{#if summary.commands.length === 0}
		<div class="empty">Aucune commande exécutée.</div>
	{:else}
		<div class="table-scroll">
		<table>
			<thead>
				<tr>
					<th>Commande</th>
					<th class="num">Coût</th>
					<th class="num">Invocations</th>
					<th>Consommation</th>
				</tr>
			</thead>
			<tbody>
				{#each summary.commands as c}
					<tr>
						<td class="mono">{c.command}</td>
						<td class="num mono">{fmt(c.total_usd)}</td>
						<td class="num mono">{c.step_count}</td>
						<td>
							<div class="bar">
								<div class="bar-fill" style:width={barWidth(c.total_usd, maxCmdUSD)}></div>
							</div>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
		</div>
	{/if}
{/if}

<style>
	.sub {
		color: var(--text-muted);
		margin: 0 0 1.5rem;
		font-size: 0.85rem;
	}
	.summary-card {
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 1.25rem 1.5rem;
		margin-bottom: 1.5rem;
	}
	.summary-row {
		display: flex;
		gap: 2rem;
		align-items: center;
		flex-wrap: wrap;
	}
	.rate { display: flex; flex-direction: column; gap: 0.1rem; }
	.rate-big {
		font-family: ui-monospace, monospace;
		font-size: 1.1rem;
		color: var(--warn);
		font-weight: 600;
	}
	.rate-big .unit { color: var(--text-muted); font-size: 0.8rem; margin-left: 0.15rem; }
	.csv-btn {
		margin-left: auto;
		padding: 0.5rem 0.9rem;
		border: 1px solid var(--border);
		border-radius: 4px;
		color: var(--text);
		text-decoration: none;
		font-size: 0.8rem;
	}
	.csv-btn:hover { border-color: var(--accent); color: var(--accent); }
	.big {
		font-size: 2.2rem;
		font-weight: 700;
		font-family: ui-monospace, monospace;
		color: var(--accent);
	}
	.muted { color: var(--text-muted); font-size: 0.8rem; }
	.num { text-align: right; }
	.mono { font-family: ui-monospace, monospace; font-size: 0.82rem; }
	.status {
		padding: 2px 8px;
		border-radius: 4px;
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		background: var(--bg-hover);
	}
	.s-shipped { background: color-mix(in srgb, var(--ok) 20%, transparent); color: var(--ok); }
	.s-failed  { background: color-mix(in srgb, var(--err) 20%, transparent); color: var(--err); }
	.s-planning, .s-building {
		background: color-mix(in srgb, var(--accent) 20%, transparent);
		color: var(--accent);
	}
	.bar {
		height: 8px;
		background: var(--bg-hover);
		border-radius: 4px;
		overflow: hidden;
		min-width: 120px;
	}
	.bar-fill {
		height: 100%;
		background: var(--accent);
		transition: width 0.3s;
	}
	.bar-fill.over-cap {
		background: var(--err);
	}
	.fail-tag {
		margin-left: 0.4rem;
		font-size: 0.65rem;
		padding: 1px 6px;
		border-radius: 3px;
		background: color-mix(in srgb, var(--err) 20%, transparent);
		color: var(--err);
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}
	a { color: var(--accent); text-decoration: none; }
	a:hover { text-decoration: underline; }
</style>

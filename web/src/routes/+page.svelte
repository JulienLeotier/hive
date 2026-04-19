<script lang="ts">
	import { apiGet } from '$lib/api';
	import { createReconnectingWS, wsURL } from '$lib/ws';
	import { fmtRelative } from '$lib/format';

	type Project = {
		id: string;
		name: string;
		idea: string;
		status: string;
		created_at: string;
		updated_at: string;
		total_cost_usd?: number;
		cost_cap_usd?: number;
		paused?: boolean;
		failure_stage?: string;
	};
	type Event = {
		id: number;
		type: string;
		source: string;
		payload: string;
		created_at: string;
	};

	let projects = $state<Project[]>([]);
	let recentEvents = $state<Event[]>([]);

	async function load() {
		try {
			const [p, e] = await Promise.all([
				apiGet<Project[]>('/api/v1/projects'),
				apiGet<Event[]>('/api/v1/events?limit=20')
			]);
			projects = p ?? [];
			recentEvents = e ?? [];
		} catch {
			/* banner */
		}
	}

	$effect(() => {
		load();
		const i = setInterval(load, 10_000);
		const ws = createReconnectingWS({
			url: wsURL('/ws'),
			onmessage: (msg) => {
				try {
					const evt = JSON.parse(msg.data) as Event;
					if (!evt.type) return;
					recentEvents = [evt, ...recentEvents].slice(0, 20);
					if (evt.type.startsWith('project.') || evt.type.startsWith('story.')) {
						load();
					}
				} catch {
					/* ignore */
				}
			}
		});
		return () => {
			ws.close();
			clearInterval(i);
		};
	});

	let counts = $derived.by(() => {
		const c: Record<string, number> = {
			draft: 0, planning: 0, building: 0, review: 0, shipped: 0, failed: 0
		};
		for (const p of projects) c[p.status] = (c[p.status] ?? 0) + 1;
		return c;
	});

	let totalCostUSD = $derived(
		projects.reduce((sum, p) => sum + (p.total_cost_usd ?? 0), 0)
	);

	let active = $derived(
		projects.filter((p) => p.status === 'building' || p.status === 'planning' || p.status === 'review')
	);

	function statusColor(s: string): string {
		const map: Record<string, string> = {
			draft: 'var(--text-muted)',
			planning: 'var(--accent)',
			building: 'var(--warn)',
			review: 'var(--warn)',
			shipped: 'var(--ok)',
			failed: 'var(--err)'
		};
		return map[s] ?? 'var(--text-muted)';
	}

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
		return '·';
	}
</script>

<svelte:head><title>Hive</title></svelte:head>

<section class="hero">
	<div class="hero-inner">
		<span class="hero-badge">Usine BMAD locale</span>
		<h1 class="hero-title">Transforme une idée en produit livré.</h1>
		<p class="hero-lede">
			Décris ce que tu veux, les agents BMAD écrivent le PRD, l'architecture,
			le code, la revue et poussent les PRs — dans ton workdir, sur ton Claude Code.
		</p>
		<div class="hero-actions">
			<a href="/projects" class="btn primary">Démarrer un projet →</a>
			<a href="/costs" class="btn ghost">Voir les coûts</a>
		</div>
	</div>
</section>

<section class="stats">
	<div class="stat stat-accent">
		<span class="stat-num">{projects.length}</span>
		<span class="stat-label">projets</span>
	</div>
	<div class="stat" class:stat-warn={counts.building + counts.review > 0}>
		<span class="stat-num">{counts.building + counts.review}</span>
		<span class="stat-label">en construction</span>
	</div>
	<div class="stat" class:stat-planning={counts.planning > 0}>
		<span class="stat-num">{counts.planning}</span>
		<span class="stat-label">en planification</span>
	</div>
	<div class="stat" class:stat-ok={counts.shipped > 0}>
		<span class="stat-num">{counts.shipped}</span>
		<span class="stat-label">livrés</span>
	</div>
	{#if counts.draft > 0}
		<div class="stat">
			<span class="stat-num">{counts.draft}</span>
			<span class="stat-label">brouillons</span>
		</div>
	{/if}
	{#if counts.failed > 0}
		<div class="stat stat-err">
			<span class="stat-num">{counts.failed}</span>
			<span class="stat-label">échoués</span>
		</div>
	{/if}
	{#if totalCostUSD > 0}
		<div class="stat stat-cost">
			<span class="stat-num">${totalCostUSD.toFixed(2)}</span>
			<span class="stat-label">cumul Claude</span>
		</div>
	{/if}
</section>

{#if active.length > 0}
	<section>
		<div class="sec-head">
			<h2>En cours</h2>
			<span class="live-pulse">
				<span class="live-dot"></span>
				<span>{active.length} actif{active.length > 1 ? 's' : ''}</span>
			</span>
		</div>
		<ul class="active-grid">
			{#each active as p (p.id)}
				<li>
					<a href="/projects/{p.id}" class="active-card">
						<header class="active-head">
							<span class="status-dot" style="background:{statusColor(p.status)}"></span>
							<span class="badge" style="background:{statusColor(p.status)}">{p.status}</span>
							{#if p.paused}
								<span class="chip paused">pause</span>
							{/if}
							{#if p.failure_stage}
								<span class="chip failed">{p.failure_stage}</span>
							{/if}
							<span class="active-time">{fmtRelative(p.updated_at)}</span>
						</header>
						<strong class="active-name">{p.name}</strong>
						<p class="active-idea">{p.idea}</p>
						<footer class="active-foot">
							{#if (p.total_cost_usd ?? 0) > 0}
								<span class="cost">${(p.total_cost_usd ?? 0).toFixed(2)}{(p.cost_cap_usd ?? 0) > 0 ? ` / $${p.cost_cap_usd?.toFixed(0)}` : ''}</span>
							{/if}
							{#if (p.cost_cap_usd ?? 0) > 0 && (p.total_cost_usd ?? 0) > 0}
								{@const pct = Math.min(100, Math.round(((p.total_cost_usd ?? 0) / (p.cost_cap_usd ?? 1)) * 100))}
								<span class="mini-bar">
									<span class="mini-fill" class:warn={pct >= 80} class:critical={pct >= 100} style="width:{pct}%"></span>
								</span>
							{/if}
						</footer>
					</a>
				</li>
			{/each}
		</ul>
	</section>
{/if}

<section class="two-col">
	<div class="col">
		<h2>Comment ça marche</h2>
		<ol class="steps">
			<li>
				<span class="step-n">1</span>
				<div>
					<strong>Idée</strong>
					<span>Tu décris le produit à l'agent PM, répond aux 5 questions de cadrage.</span>
				</div>
			</li>
			<li>
				<span class="step-n">2</span>
				<div>
					<strong>Planning BMAD</strong>
					<span>Analyst → Product brief → PM → PRD → Architecture → Epics / stories.</span>
				</div>
			</li>
			<li>
				<span class="step-n">3</span>
				<div>
					<strong>Dev + revue</strong>
					<span>Pour chaque story : <code>/bmad-dev-story</code> code, <code>/bmad-code-review</code> valide, PR poussée.</span>
				</div>
			</li>
			<li>
				<span class="step-n">4</span>
				<div>
					<strong>Livraison</strong>
					<span>Quand toutes les ACs passent, le projet flippe en <code>shipped</code>.</span>
				</div>
			</li>
		</ol>
	</div>

	<div class="col">
		<h2>Activité</h2>
		{#if recentEvents.length === 0}
			<div class="empty">
				<span class="empty-icon">◌</span>
				Silence radio. Lance un projet pour voir passer les events BMAD.
			</div>
		{:else}
			<ul class="timeline">
				{#each recentEvents as e (e.id)}
					<li>
						<span class="tl-icon" style="color:{eventColor(e.type)}">{eventIcon(e.type)}</span>
						<div class="tl-body">
							<span class="tl-type" style="color:{eventColor(e.type)}">{e.type}</span>
							<span class="tl-time">{fmtRelative(e.created_at)}</span>
						</div>
					</li>
				{/each}
			</ul>
		{/if}
	</div>
</section>

<style>
	/* ===== Hero ===== */
	.hero {
		margin: -2rem -2.5rem 2rem;
		padding: 3rem 2.5rem;
		background:
			radial-gradient(ellipse at top left, color-mix(in srgb, var(--accent) 18%, transparent), transparent 60%),
			radial-gradient(ellipse at bottom right, color-mix(in srgb, var(--accent) 10%, transparent), transparent 60%),
			var(--bg-panel);
		border-bottom: 1px solid var(--border);
	}
	.hero-inner { max-width: 720px; }
	.hero-badge {
		display: inline-block;
		padding: 0.2rem 0.7rem;
		background: color-mix(in srgb, var(--accent) 18%, transparent);
		color: var(--accent);
		border-radius: 999px;
		font-size: 0.7rem;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		margin-bottom: 0.9rem;
	}
	.hero-title {
		font-size: 2.4rem;
		line-height: 1.15;
		margin: 0 0 0.7rem;
		letter-spacing: -0.02em;
		font-weight: 700;
	}
	.hero-lede {
		color: var(--text-muted);
		font-size: 1.05rem;
		line-height: 1.55;
		margin: 0 0 1.5rem;
	}
	.hero-actions { display: flex; flex-wrap: wrap; gap: 0.6rem; }
	.btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 0.6rem 1.1rem;
		border-radius: 6px;
		text-decoration: none;
		font-weight: 600;
		font-size: 0.9rem;
		transition: background 0.1s, transform 0.05s, border-color 0.1s;
	}
	.btn:active { transform: translateY(1px); }
	.btn.primary {
		background: var(--accent);
		color: white;
		border: 1px solid var(--accent);
	}
	.btn.primary:hover { background: color-mix(in srgb, var(--accent) 88%, black); }
	.btn.ghost {
		background: transparent;
		color: var(--text);
		border: 1px solid var(--border);
	}
	.btn.ghost:hover { border-color: var(--accent); color: var(--accent); }

	/* ===== Stats ===== */
	.stats {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
		gap: 0.75rem;
		margin-bottom: 2rem;
	}
	.stat {
		padding: 1rem 1.1rem;
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 8px;
		display: flex;
		flex-direction: column;
		gap: 0.15rem;
		position: relative;
		overflow: hidden;
	}
	.stat::before {
		content: '';
		position: absolute;
		top: 0;
		left: 0;
		width: 3px;
		height: 100%;
		background: var(--text-muted);
		opacity: 0.3;
	}
	.stat-accent::before { background: var(--accent); opacity: 1; }
	.stat-warn::before { background: var(--warn); opacity: 1; }
	.stat-planning::before { background: var(--accent); opacity: 1; }
	.stat-ok::before { background: var(--ok); opacity: 1; }
	.stat-err::before { background: var(--err); opacity: 1; }
	.stat-cost::before { background: var(--warn); opacity: 1; }
	.stat-num {
		font-size: 1.75rem;
		font-weight: 700;
		line-height: 1;
		font-variant-numeric: tabular-nums;
		letter-spacing: -0.02em;
	}
	.stat-cost .stat-num {
		font-family: ui-monospace, monospace;
		font-size: 1.4rem;
		color: var(--warn);
	}
	.stat-label {
		font-size: 0.72rem;
		color: var(--text-muted);
		text-transform: uppercase;
		letter-spacing: 0.06em;
		margin-top: 0.2rem;
	}

	/* ===== Sections ===== */
	section { margin-bottom: 2rem; }
	section :global(h2) {
		font-size: 0.78rem;
		text-transform: uppercase;
		letter-spacing: 0.1em;
		color: var(--text-muted);
		margin: 0 0 0.75rem;
		font-weight: 700;
	}
	.sec-head {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 0.75rem;
	}
	.sec-head :global(h2) { margin: 0; }
	.live-pulse {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		font-size: 0.75rem;
		color: var(--warn);
		font-weight: 600;
	}
	.live-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background: var(--warn);
		animation: pulse 1.2s ease-in-out infinite;
	}
	@keyframes pulse {
		0%, 100% { opacity: 1; transform: scale(1); }
		50% { opacity: 0.5; transform: scale(0.85); }
	}

	/* ===== Active projects ===== */
	.active-grid {
		list-style: none;
		padding: 0;
		margin: 0;
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
		gap: 0.8rem;
	}
	.active-grid li a {
		display: flex;
		flex-direction: column;
		gap: 0.4rem;
		padding: 0.9rem 1rem;
		border: 1px solid var(--border);
		border-radius: 8px;
		background: var(--bg-panel);
		color: var(--text);
		text-decoration: none;
		transition: border-color 0.1s, transform 0.1s;
	}
	.active-grid li a:hover {
		border-color: var(--accent);
		transform: translateY(-1px);
	}
	.active-head {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		flex-wrap: wrap;
	}
	.active-time {
		margin-left: auto;
		font-size: 0.72rem;
		color: var(--text-muted);
	}
	.active-name {
		font-size: 1.02rem;
	}
	.active-idea {
		font-size: 0.82rem;
		color: var(--text-muted);
		margin: 0;
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}
	.active-foot {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		margin-top: auto;
	}
	.active-foot .cost {
		font-size: 0.78rem;
		font-variant-numeric: tabular-nums;
		color: var(--text-muted);
	}
	.mini-bar {
		flex: 1;
		height: 3px;
		background: var(--border);
		border-radius: 2px;
		overflow: hidden;
		min-width: 40px;
	}
	.mini-fill {
		display: block;
		height: 100%;
		background: var(--accent);
		transition: width 0.3s, background 0.2s;
	}
	.mini-fill.warn { background: #f59e0b; }
	.mini-fill.critical { background: var(--err); }
	.chip.paused {
		background: color-mix(in srgb, var(--accent) 15%, transparent);
		color: var(--accent);
		border-radius: 10px;
		padding: 1px 8px;
		font-size: 0.68rem;
		font-weight: 600;
	}
	.chip.failed {
		background: color-mix(in srgb, var(--err) 15%, transparent);
		color: var(--err);
		border-radius: 10px;
		padding: 1px 8px;
		font-size: 0.68rem;
		font-weight: 600;
	}

	.status-dot {
		width: 10px;
		height: 10px;
		border-radius: 50%;
		flex-shrink: 0;
		box-shadow: 0 0 12px currentColor;
	}

	/* ===== Two-column layout ===== */
	.two-col {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 1.5rem;
	}
	.col { min-width: 0; }

	/* ===== Steps ===== */
	.steps {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}
	.steps li {
		display: flex;
		gap: 0.85rem;
		padding: 0.8rem 1rem;
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 8px;
	}
	.step-n {
		width: 28px;
		height: 28px;
		border-radius: 50%;
		background: color-mix(in srgb, var(--accent) 18%, transparent);
		color: var(--accent);
		font-size: 0.8rem;
		font-weight: 700;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
	}
	.steps strong {
		display: block;
		font-size: 0.95rem;
		margin-bottom: 0.15rem;
	}
	.steps span {
		font-size: 0.82rem;
		color: var(--text-muted);
		line-height: 1.5;
	}
	.steps code {
		background: var(--bg-hover);
		padding: 1px 5px;
		border-radius: 3px;
		font-size: 0.78rem;
		color: var(--accent);
	}

	/* ===== Timeline ===== */
	.timeline {
		list-style: none;
		padding: 0;
		margin: 0;
		max-height: 400px;
		overflow-y: auto;
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
	}
	.timeline li {
		display: flex;
		gap: 0.7rem;
		align-items: flex-start;
		padding: 0.5rem 0.75rem;
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 6px;
	}
	.tl-icon {
		font-size: 0.9rem;
		font-family: ui-monospace, monospace;
		width: 18px;
		text-align: center;
		flex-shrink: 0;
		line-height: 1.4;
	}
	.tl-body {
		flex: 1;
		min-width: 0;
		display: flex;
		justify-content: space-between;
		align-items: baseline;
		gap: 0.5rem;
	}
	.tl-type {
		font-family: ui-monospace, monospace;
		font-size: 0.78rem;
		font-weight: 600;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		min-width: 0;
	}
	.tl-time {
		font-size: 0.72rem;
		color: var(--text-muted);
		white-space: nowrap;
		flex-shrink: 0;
	}

	/* ===== Empty state ===== */
	.empty {
		padding: 2rem 1rem;
		text-align: center;
		color: var(--text-muted);
		background: var(--bg-panel);
		border: 1px dashed var(--border);
		border-radius: 8px;
		font-style: italic;
	}
	.empty-icon {
		display: block;
		font-size: 2rem;
		margin-bottom: 0.5rem;
		font-style: normal;
		opacity: 0.5;
	}

	.badge {
		display: inline-block;
		padding: 0.15rem 0.55rem;
		border-radius: 4px;
		color: white;
		font-size: 0.68rem;
		font-weight: 600;
		text-transform: lowercase;
		letter-spacing: 0.03em;
	}

	/* ===== Responsive ===== */
	@media (max-width: 767px) {
		.hero {
			margin: -1rem -1rem 1.5rem;
			padding: 2rem 1rem;
		}
		.hero-title { font-size: 1.7rem; }
		.hero-lede { font-size: 0.95rem; }
		.hero-badge { margin-bottom: 0.7rem; }
		.stats { grid-template-columns: repeat(2, 1fr); gap: 0.5rem; }
		.stat { padding: 0.75rem; }
		.stat-num { font-size: 1.4rem; }
		.two-col { grid-template-columns: 1fr; gap: 1rem; }
		.timeline { max-height: none; }
		.active-idea { font-size: 0.78rem; }
	}
	@media (min-width: 768px) and (max-width: 1023px) {
		.hero { margin: -1.5rem -1.75rem 2rem; padding: 2.5rem 1.75rem; }
		.two-col { grid-template-columns: 1fr; }
	}
</style>

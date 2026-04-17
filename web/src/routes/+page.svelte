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
				apiGet<Event[]>('/api/v1/events?limit=15')
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
					recentEvents = [evt, ...recentEvents].slice(0, 15);
					// Project status may have changed — cheap refresh.
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
</script>

<main>
	<header class="hero">
		<h1>Hive</h1>
		<p class="tagline">
			Local BMAD product factory — describe what you want built, the agents
			turn it into a shipped product in your workdir.
		</p>
		<a href="/projects" class="cta">Start a new project →</a>
	</header>

	<section class="flow">
		<h2>How it works</h2>
		<ol class="steps">
			<li><strong>Idea</strong><span>Tell the PM agent what you want.</span></li>
			<li><strong>PRD</strong><span>It asks until it has enough, emits a PRD.</span></li>
			<li><strong>Architect</strong><span>Decomposes into epics, stories, acceptance criteria.</span></li>
			<li><strong>Dev + Review</strong><span>Claude Code writes, a reviewer checks every AC, loops until green.</span></li>
			<li><strong>Ship</strong><span>Every commit lands in your workdir. When every AC passes, the project flips to shipped.</span></li>
		</ol>
	</section>

	<section class="summary">
		<h2>Fleet</h2>
		<div class="cards">
			<div class="card"><strong>{projects.length}</strong><span>projects total</span></div>
			<div class="card"><strong style="color:var(--warn)">{counts.building + counts.review}</strong><span>building</span></div>
			<div class="card"><strong style="color:var(--accent)">{counts.planning}</strong><span>planning</span></div>
			<div class="card"><strong style="color:var(--ok)">{counts.shipped}</strong><span>shipped</span></div>
			<div class="card"><strong>{counts.draft}</strong><span>draft</span></div>
			{#if counts.failed > 0}
				<div class="card"><strong style="color:var(--err)">{counts.failed}</strong><span>failed</span></div>
			{/if}
		</div>
	</section>

	{#if active.length > 0}
		<section>
			<h2>Active now</h2>
			<ul class="active-list">
				{#each active as p (p.id)}
					<li>
						<a href="/projects/{p.id}">
							<span class="badge" style="background:{statusColor(p.status)}">{p.status}</span>
							<strong>{p.name}</strong>
							<span class="muted">{p.idea}</span>
						</a>
					</li>
				{/each}
			</ul>
		</section>
	{/if}

	<section>
		<h2>Recent events</h2>
		{#if recentEvents.length === 0}
			<p class="empty">Quiet. Start a project and come back.</p>
		{:else}
			<ul class="events">
				{#each recentEvents as e (e.id)}
					<li>
						<span class="t" style="color:{e.type.startsWith('project.shipped') || e.type === 'story.reviewed' ? 'var(--ok)' : e.type.endsWith('.failed') || e.type === 'story.blocked' ? 'var(--err)' : 'var(--accent)'}">{e.type}</span>
						<span class="muted">{fmtRelative(e.created_at)}</span>
					</li>
				{/each}
			</ul>
		{/if}
	</section>
</main>

<style>
	main {
		display: flex;
		flex-direction: column;
		gap: 2rem;
		max-width: 960px;
	}
	.hero {
		padding: 1.5rem 0;
		border-bottom: 1px solid var(--border);
	}
	.hero h1 {
		font-size: 2.5rem;
		margin: 0 0 0.25rem;
	}
	.tagline {
		color: var(--text-muted);
		margin: 0 0 1rem;
		font-size: 1rem;
		line-height: 1.5;
	}
	.cta {
		display: inline-block;
		padding: 0.55rem 1rem;
		background: var(--accent);
		color: white;
		border-radius: 6px;
		text-decoration: none;
		font-weight: 600;
		font-size: 0.9rem;
	}
	.cta:hover { background: color-mix(in srgb, var(--accent) 85%, black); }

	.flow h2, .summary h2, section h2 {
		font-size: 0.85rem;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--text-muted);
		margin: 0 0 0.75rem;
	}
	.steps {
		list-style: none;
		padding: 0;
		margin: 0;
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
		gap: 0.6rem;
		counter-reset: step;
	}
	.steps li {
		position: relative;
		padding: 0.75rem 0.85rem 0.75rem 2.25rem;
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 6px;
		counter-increment: step;
	}
	.steps li::before {
		content: counter(step);
		position: absolute;
		left: 0.75rem;
		top: 0.7rem;
		width: 1.25rem;
		height: 1.25rem;
		border-radius: 50%;
		background: var(--accent);
		color: white;
		font-size: 0.7rem;
		font-weight: 700;
		display: inline-flex;
		align-items: center;
		justify-content: center;
	}
	.steps strong {
		display: block;
		font-size: 0.9rem;
		margin-bottom: 0.15rem;
	}
	.steps span {
		font-size: 0.78rem;
		color: var(--text-muted);
		line-height: 1.35;
	}

	.cards {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
		gap: 0.75rem;
	}
	.card {
		padding: 0.9rem 1rem;
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 6px;
	}
	.card strong {
		display: block;
		font-size: 1.75rem;
		font-weight: 600;
	}
	.card span {
		font-size: 0.75rem;
		color: var(--text-muted);
	}

	.active-list {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
	}
	.active-list li a {
		display: flex;
		gap: 0.6rem;
		align-items: center;
		padding: 0.55rem 0.75rem;
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 6px;
		text-decoration: none;
		color: inherit;
	}
	.active-list li a:hover { border-color: var(--accent); }
	.active-list .muted {
		color: var(--text-muted);
		font-size: 0.82rem;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.events {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
		font-size: 0.82rem;
	}
	.events li {
		display: flex;
		gap: 0.75rem;
		align-items: baseline;
	}
	.events .t { font-weight: 600; min-width: 12rem; }
	.muted { color: var(--text-muted); font-size: 0.8rem; }
	.empty { color: var(--text-muted); font-style: italic; }
	.badge {
		display: inline-block;
		padding: 0.12rem 0.45rem;
		border-radius: 4px;
		color: white;
		font-size: 0.68rem;
		font-weight: 500;
	}
</style>

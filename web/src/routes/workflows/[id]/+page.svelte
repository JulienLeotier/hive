<script lang="ts">
	import { page } from '$app/stores';
	import { apiGet } from '$lib/api';
	import { fmtDuration, fmtRelative, truncate } from '$lib/format';

	type TaskDetail = {
		id: string;
		type: string;
		status: string;
		agent_id?: string;
		agent_name?: string;
		input?: string;
		output?: string;
		error?: string;
		created_at: string;
		started_at?: string;
		completed_at?: string;
	};
	type Run = {
		id: string;
		name: string;
		status: string;
		config: string;
		created_at: string;
		tasks: TaskDetail[];
	};

	let run = $state<Run | null>(null);
	let loading = $state(true);
	let selected = $state<TaskDetail | null>(null);

	$effect(() => {
		loading = true;
		const id = $page.params.id ?? '';
		apiGet<Run>(`/api/v1/workflows/${encodeURIComponent(id)}`)
			.then((r) => {
				run = r;
				loading = false;
			})
			.catch(() => {
				loading = false;
			});
	});

	function statusColor(s: string): string {
		const map: Record<string, string> = {
			pending: 'var(--text-muted)',
			assigned: 'var(--accent)',
			running: 'var(--warn)',
			completed: 'var(--ok)',
			failed: 'var(--err)',
			idle: 'var(--text-muted)'
		};
		return map[s] ?? 'var(--text-muted)';
	}

	function durationSeconds(t: TaskDetail): number {
		if (!t.started_at || !t.completed_at) return 0;
		const s = new Date(t.started_at).getTime();
		const e = new Date(t.completed_at).getTime();
		if (isNaN(s) || isNaN(e)) return 0;
		return (e - s) / 1000;
	}

	function prettyJSON(raw: string): string {
		if (!raw) return '';
		try {
			return JSON.stringify(JSON.parse(raw), null, 2);
		} catch {
			return raw;
		}
	}
</script>

<main>
	<a class="back" href="/workflows">← all workflows</a>

	{#if loading}
		<p class="empty">Loading run…</p>
	{:else if !run}
		<p class="empty">Run not found.</p>
	{:else}
		<header>
			<h1>{run.name}</h1>
			<div class="meta">
				<span class="badge" style="background:{statusColor(run.status)}">{run.status}</span>
				<span class="muted">started {fmtRelative(run.created_at)}</span>
				<code class="id">{run.id}</code>
			</div>
		</header>

		<section class="timeline">
			<h2>Tasks ({run.tasks?.length ?? 0})</h2>
			{#if !run.tasks || run.tasks.length === 0}
				<p class="empty">No tasks recorded for this run yet.</p>
			{:else}
				<ol class="tasks">
					{#each run.tasks as t (t.id)}
						<li class:selected={selected?.id === t.id} class:failed={t.status === 'failed'}>
							<button class="task-btn" onclick={() => (selected = t)}>
								<span class="dot" style="background:{statusColor(t.status)}"></span>
								<div class="body">
									<div class="row1">
										<strong>{t.type}</strong>
										<span class="muted">{t.agent_name || '—'}</span>
									</div>
									<div class="row2">
										<span class="badge" style="background:{statusColor(t.status)}">{t.status}</span>
										<span class="muted">{fmtDuration(durationSeconds(t))}</span>
										<code class="taskid">{t.id.slice(-12)}</code>
									</div>
								</div>
							</button>
						</li>
					{/each}
				</ol>
			{/if}
		</section>

		{#if selected}
			<aside class="detail">
				<h3>{selected.type} <code class="taskid">{selected.id}</code></h3>
				{#if selected.error}
					<h4>Error</h4>
					<pre class="err-block">{selected.error}</pre>
				{/if}
				{#if selected.input}
					<h4>Input</h4>
					<pre>{prettyJSON(selected.input)}</pre>
				{/if}
				{#if selected.output && !selected.error}
					<h4>Output</h4>
					<pre>{prettyJSON(selected.output)}</pre>
				{/if}
				<h4>Timeline</h4>
				<dl>
					<dt>Created</dt>
					<dd>{selected.created_at}</dd>
					{#if selected.started_at}
						<dt>Started</dt>
						<dd>{selected.started_at}</dd>
					{/if}
					{#if selected.completed_at}
						<dt>Completed</dt>
						<dd>{selected.completed_at}</dd>
					{/if}
				</dl>
			</aside>
		{:else if run && run.tasks && run.tasks.length > 0}
			<aside class="hint">Click a task on the left to see its input, output, and timing.</aside>
		{/if}

		<section class="config">
			<h2>Workflow config</h2>
			<pre>{prettyJSON(run.config)}</pre>
		</section>
	{/if}
</main>

<style>
	main {
		display: grid;
		grid-template-columns: 1fr 1fr;
		grid-template-rows: auto auto auto auto;
		gap: 1.5rem;
		grid-template-areas:
			'back back'
			'header header'
			'timeline detail'
			'config config';
	}
	.back { grid-area: back; color: var(--muted); text-decoration: none; font-size: 0.85rem; }
	.back:hover { color: var(--accent); }
	header { grid-area: header; }
	.timeline { grid-area: timeline; }
	.detail, .hint { grid-area: detail; }
	.config { grid-area: config; }
	h1 { margin: 0 0 0.5rem 0; }
	h2 { font-size: 1.05rem; margin: 0 0 0.75rem 0; }
	h3 { font-size: 0.95rem; margin: 0 0 0.5rem 0; }
	h4 { font-size: 0.75rem; text-transform: uppercase; color: var(--muted); margin: 1rem 0 0.3rem 0; }
	.meta { display: flex; gap: 0.75rem; align-items: center; flex-wrap: wrap; font-size: 0.85rem; }
	.muted { color: var(--muted); }
	.id { font-size: 0.75rem; color: var(--muted); }
	.badge {
		display: inline-block;
		padding: 0.125rem 0.5rem;
		border-radius: 4px;
		color: white;
		font-size: 0.7rem;
		font-weight: 500;
	}
	.empty { color: var(--muted); font-style: italic; }
	.tasks {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}
	.tasks li {
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 6px;
		overflow: hidden;
		transition: background 0.1s;
	}
	.tasks li:hover { background: var(--bg-hover); }
	.tasks li.selected { border-color: var(--accent); background: var(--bg-hover); }
	.tasks li.failed { border-left: 3px solid var(--err); }
	.task-btn {
		width: 100%;
		display: flex;
		gap: 0.75rem;
		padding: 0.6rem 0.75rem;
		background: transparent;
		color: inherit;
		border: none;
		text-align: left;
		cursor: pointer;
		font: inherit;
	}
	.dot {
		width: 10px;
		height: 10px;
		border-radius: 50%;
		margin-top: 6px;
		flex: none;
	}
	.body { flex: 1; display: flex; flex-direction: column; gap: 0.3rem; }
	.row1, .row2 { display: flex; gap: 0.5rem; align-items: center; font-size: 0.85rem; }
	.taskid { font-size: 0.7rem; color: var(--muted); }
	.detail, .hint {
		padding: 1rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 6px;
		align-self: start;
	}
	.hint { color: var(--muted); font-style: italic; }
	pre {
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 4px;
		padding: 0.6rem;
		font-size: 0.75rem;
		overflow-x: auto;
		max-height: 260px;
		white-space: pre-wrap;
		word-break: break-word;
	}
	.err-block { border-left: 3px solid var(--err); }
	dl { display: grid; grid-template-columns: max-content 1fr; gap: 0.25rem 0.75rem; font-size: 0.8rem; margin: 0; }
	dt { color: var(--muted); }
	dd { margin: 0; font-family: ui-monospace, monospace; }
</style>

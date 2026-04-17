<script lang="ts">
	import { page } from '$app/stores';
	import { apiGet } from '$lib/api';
	import { fmtRelative } from '$lib/format';

	type AcceptanceCriterion = {
		id: number;
		text: string;
		passed: boolean;
		ordering: number;
	};
	type Story = {
		id: string;
		title: string;
		description?: string;
		status: string;
		iterations: number;
		agent_id?: string;
		branch?: string;
		acceptance_criteria?: AcceptanceCriterion[];
	};
	type Epic = {
		id: string;
		title: string;
		description?: string;
		status: string;
		stories?: Story[];
	};
	type Project = {
		id: string;
		name: string;
		idea: string;
		prd?: string;
		workdir?: string;
		bmad_output_path?: string;
		repo_path?: string;
		status: string;
		created_at: string;
		updated_at: string;
		epics?: Epic[];
	};

	let project = $state<Project | null>(null);
	let loading = $state(true);

	async function load() {
		const id = $page.params.id ?? '';
		if (!id) return;
		try {
			project = await apiGet<Project>(`/api/v1/projects/${encodeURIComponent(id)}`);
		} catch {
			/* banner */
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		load();
		const i = setInterval(load, 5000);
		return () => clearInterval(i);
	});

	function statusColor(s: string): string {
		const map: Record<string, string> = {
			draft: 'var(--text-muted)',
			planning: 'var(--accent)',
			building: 'var(--warn)',
			review: 'var(--warn)',
			shipped: 'var(--ok)',
			failed: 'var(--err)',
			pending: 'var(--text-muted)',
			in_progress: 'var(--warn)',
			done: 'var(--ok)',
			blocked: 'var(--err)',
			dev: 'var(--warn)',
			qa: 'var(--accent)'
		};
		return map[s] ?? 'var(--text-muted)';
	}

	let totalStories = $derived(
		(project?.epics ?? []).reduce((n, e) => n + (e.stories?.length ?? 0), 0)
	);
	let doneStories = $derived(
		(project?.epics ?? []).reduce(
			(n, e) => n + (e.stories?.filter((s) => s.status === 'done').length ?? 0),
			0
		)
	);
	let totalACs = $derived(
		(project?.epics ?? []).reduce(
			(n, e) =>
				n +
				(e.stories?.reduce((m, s) => m + (s.acceptance_criteria?.length ?? 0), 0) ?? 0),
			0
		)
	);
	let passedACs = $derived(
		(project?.epics ?? []).reduce(
			(n, e) =>
				n +
				(e.stories?.reduce(
					(m, s) => m + (s.acceptance_criteria?.filter((ac) => ac.passed).length ?? 0),
					0
				) ?? 0),
			0
		)
	);
</script>

<main>
	<a class="back" href="/projects">← all projects</a>

	{#if loading}
		<p class="empty">Loading project…</p>
	{:else if !project}
		<p class="empty">Project not found.</p>
	{:else}
		<header>
			<h1>{project.name}</h1>
			<div class="meta">
				<span class="badge" style="background:{statusColor(project.status)}">{project.status}</span>
				<span class="muted">updated {fmtRelative(project.updated_at)}</span>
				<code class="id">{project.id}</code>
			</div>
			<p class="idea">{project.idea}</p>
			{#if project.bmad_output_path || project.repo_path || project.workdir}
				<dl class="refs">
					{#if project.workdir}
						<dt>Workdir</dt><dd><code>{project.workdir}</code></dd>
					{/if}
					{#if project.repo_path}
						<dt>Existing repo</dt><dd><code>{project.repo_path}</code></dd>
					{/if}
					{#if project.bmad_output_path}
						<dt>BMAD output</dt><dd><code>{project.bmad_output_path}</code></dd>
					{/if}
				</dl>
			{/if}
		</header>

		<section class="progress">
			<h2>Progress</h2>
			<div class="metrics">
				<div><strong>{doneStories}/{totalStories}</strong><span>stories done</span></div>
				<div><strong>{passedACs}/{totalACs}</strong><span>acceptance criteria passed</span></div>
				<div><strong>{project.epics?.length ?? 0}</strong><span>epics</span></div>
			</div>
		</section>

		{#if !project.prd}
			<section class="panel info">
				<h3>Waiting for the PM agent</h3>
				<p>
					This project is in <code>{project.status}</code>. The PM agent will start an
					interactive Q&amp;A here to turn your idea into a PRD. That flow comes online in
					the next phase of the BMAD pivot — right now you've got the project record and
					schema, but the autonomous build loop isn't wired yet.
				</p>
				<p class="idea-recap"><strong>Your idea:</strong> {project.idea}</p>
			</section>
		{:else}
			<section class="panel">
				<h3>PRD</h3>
				<pre class="prd">{project.prd}</pre>
			</section>
		{/if}

		<section class="tree">
			<h2>Work breakdown</h2>
			{#if !project.epics || project.epics.length === 0}
				<p class="empty">No epics yet. The Architect agent will emit them once the PRD is locked.</p>
			{:else}
				{#each project.epics as epic (epic.id)}
					<div class="epic">
						<header>
							<h3>{epic.title}</h3>
							<span class="badge" style="background:{statusColor(epic.status)}">{epic.status}</span>
						</header>
						{#if epic.description}
							<p class="desc">{epic.description}</p>
						{/if}
						{#if epic.stories && epic.stories.length > 0}
							<ul class="stories">
								{#each epic.stories as story (story.id)}
									<li>
										<div class="story-head">
											<strong>{story.title}</strong>
											<span class="badge" style="background:{statusColor(story.status)}">{story.status}</span>
											{#if story.iterations > 0}
												<span class="muted">· {story.iterations} iteration{story.iterations > 1 ? 's' : ''}</span>
											{/if}
										</div>
										{#if story.acceptance_criteria && story.acceptance_criteria.length > 0}
											<ul class="acs">
												{#each story.acceptance_criteria as ac (ac.id)}
													<li class:passed={ac.passed}>
														<span class="check">{ac.passed ? '✓' : '○'}</span>
														{ac.text}
													</li>
												{/each}
											</ul>
										{/if}
									</li>
								{/each}
							</ul>
						{/if}
					</div>
				{/each}
			{/if}
		</section>
	{/if}
</main>

<style>
	main {
		display: flex;
		flex-direction: column;
		gap: 1.5rem;
		max-width: 1000px;
	}
	.back {
		color: var(--muted);
		text-decoration: none;
		font-size: 0.85rem;
	}
	.back:hover { color: var(--accent); }
	h1 { margin: 0 0 0.5rem 0; }
	h2 { font-size: 1.05rem; margin: 0 0 0.75rem 0; }
	h3 { font-size: 0.95rem; margin: 0 0 0.5rem 0; }
	.meta {
		display: flex;
		gap: 0.75rem;
		align-items: center;
		flex-wrap: wrap;
		font-size: 0.85rem;
	}
	.muted { color: var(--muted); }
	.id { font-size: 0.75rem; color: var(--muted); }
	.idea {
		margin: 0.5rem 0 0;
		font-size: 1rem;
		color: var(--text);
		line-height: 1.5;
	}
	.refs {
		display: grid;
		grid-template-columns: max-content 1fr;
		column-gap: 0.75rem;
		row-gap: 0.15rem;
		margin: 0.75rem 0 0;
		font-size: 0.8rem;
	}
	.refs dt { color: var(--muted); }
	.refs dd { margin: 0; font-family: ui-monospace, monospace; }
	.badge {
		display: inline-block;
		padding: 0.125rem 0.5rem;
		border-radius: 4px;
		color: white;
		font-size: 0.7rem;
		font-weight: 500;
	}
	.empty {
		color: var(--muted);
		font-style: italic;
	}
	.metrics {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
		gap: 1rem;
	}
	.metrics div {
		padding: 1rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 6px;
	}
	.metrics strong {
		display: block;
		font-size: 1.5rem;
		margin-bottom: 0.2rem;
	}
	.metrics span {
		font-size: 0.8rem;
		color: var(--muted);
	}
	.panel {
		padding: 1rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 6px;
	}
	.panel.info {
		border-left: 3px solid var(--accent);
	}
	.panel p { margin: 0.5rem 0; }
	.idea-recap {
		margin-top: 0.75rem;
		padding: 0.6rem;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 4px;
		font-size: 0.85rem;
	}
	.prd {
		white-space: pre-wrap;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 4px;
		padding: 0.75rem;
		font-size: 0.85rem;
		overflow-x: auto;
	}
	.epic {
		padding: 1rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 6px;
		margin-bottom: 0.75rem;
	}
	.epic header {
		display: flex;
		justify-content: space-between;
		align-items: center;
	}
	.desc {
		color: var(--muted);
		font-size: 0.9rem;
		margin: 0.4rem 0 0.75rem 0;
	}
	.stories {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}
	.stories li {
		padding: 0.6rem 0.75rem;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 4px;
	}
	.story-head {
		display: flex;
		gap: 0.5rem;
		align-items: center;
		font-size: 0.9rem;
	}
	.acs {
		list-style: none;
		padding: 0;
		margin: 0.5rem 0 0 0;
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
		font-size: 0.85rem;
	}
	.acs li {
		padding: 0.2rem 0;
		background: transparent;
		border: none;
		border-radius: 0;
		color: var(--muted);
	}
	.acs li.passed {
		color: var(--ok);
	}
	.acs .check {
		display: inline-block;
		width: 1rem;
		margin-right: 0.4rem;
	}
</style>

<script lang="ts">
	import { page } from '$app/stores';
	import { apiGet, apiPost } from '$lib/api';
	import { fmtRelative } from '$lib/format';
	import { createReconnectingWS, wsURL } from '$lib/ws';

	type IntakeMessage = {
		id: number;
		author: string;
		content: string;
		created_at: string;
	};
	type Conversation = {
		id: string;
		project_id: string;
		role: string;
		status: string;
		messages?: IntakeMessage[];
	};

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

	// Intake state
	let conversation = $state<Conversation | null>(null);
	let intakeLoading = $state(false);
	let replyDraft = $state('');
	let sending = $state(false);
	let intakeDone = $state(false);
	let finalizing = $state(false);
	let intakeError = $state('');

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

	async function loadIntake() {
		const id = $page.params.id ?? '';
		if (!id || !project || project.status !== 'draft') return;
		intakeLoading = true;
		try {
			conversation = await apiGet<Conversation>(
				`/api/v1/projects/${encodeURIComponent(id)}/intake`
			);
		} catch (e) {
			intakeError = e instanceof Error ? e.message : String(e);
		} finally {
			intakeLoading = false;
		}
	}

	async function sendReply() {
		const id = $page.params.id ?? '';
		if (!id || !replyDraft.trim()) return;
		intakeError = '';
		sending = true;
		try {
			const resp = (await apiPost(
				`/api/v1/projects/${encodeURIComponent(id)}/intake/messages`,
				{ content: replyDraft }
			)) as { conversation: Conversation; done: boolean };
			conversation = resp.conversation;
			intakeDone = resp.done;
			replyDraft = '';
		} catch (e) {
			intakeError = e instanceof Error ? e.message : String(e);
		} finally {
			sending = false;
		}
	}

	async function finalizePRD() {
		const id = $page.params.id ?? '';
		if (!id) return;
		intakeError = '';
		finalizing = true;
		try {
			await apiPost(`/api/v1/projects/${encodeURIComponent(id)}/intake/finalize`, {});
			await load();
		} catch (e) {
			intakeError = e instanceof Error ? e.message : String(e);
		} finally {
			finalizing = false;
		}
	}

	// Adaptive polling: a project that's actively building can move in
	// seconds (devloop ticks + Claude Code finishes a story), so we poll
	// fast. Draft/shipped projects barely change; poll slowly so we're
	// not hammering SQLite for no reason.
	$effect(() => {
		load();
		const fast = project?.status === 'building' || project?.status === 'review';
		const intervalMs = fast ? 2000 : 10000;
		const i = setInterval(load, intervalMs);
		return () => clearInterval(i);
	});

	// WebSocket subscription: the devloop emits story.* and project.shipped
	// events as it works. Reacting to them eliminates polling lag — the
	// UI updates the moment a story flips dev → review → done.
	$effect(() => {
		const ws = createReconnectingWS({
			url: wsURL('/ws'),
			onmessage: (msg) => {
				try {
					const evt = JSON.parse(msg.data) as { type?: string; payload?: string };
					if (!evt.type) return;
					if (!evt.type.startsWith('story.') && evt.type !== 'project.shipped') return;
					// Payload is a JSON string on the wire; parse and match project_id.
					const pid = $page.params.id ?? '';
					let payloadProject = '';
					try {
						const p = JSON.parse(evt.payload ?? '{}') as { project_id?: string };
						payloadProject = p.project_id ?? '';
					} catch {
						return;
					}
					if (payloadProject && payloadProject === pid) load();
				} catch {
					/* ignore non-JSON frames */
				}
			}
		});
		return () => ws.close();
	});

	// Load the intake conversation whenever we're on a draft project.
	$effect(() => {
		if (project && project.status === 'draft' && !conversation) {
			loadIntake();
		}
	});

	function isActive(s: string): boolean {
		return s === 'dev' || s === 'review' || s === 'in_progress';
	}

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
			{#if totalACs > 0}
				<div class="bar" aria-label="acceptance criteria progress">
					<div
						class="bar-fill"
						class:shipped={project.status === 'shipped'}
						style="width:{Math.round((passedACs / totalACs) * 100)}%"
					></div>
					<span class="bar-label">
						{passedACs}/{totalACs} ACs · {Math.round((passedACs / totalACs) * 100)}%
					</span>
				</div>
			{/if}
			<div class="metrics">
				<div><strong>{doneStories}/{totalStories}</strong><span>stories done</span></div>
				<div><strong>{passedACs}/{totalACs}</strong><span>acceptance criteria passed</span></div>
				<div><strong>{project.epics?.length ?? 0}</strong><span>epics</span></div>
			</div>
		</section>

		{#if project.status === 'draft'}
			<section class="intake">
				<h2>Intake — talk to the PM agent</h2>
				{#if intakeLoading && !conversation}
					<p class="empty">Starting the conversation…</p>
				{:else if conversation}
					<div class="chat">
						{#each conversation.messages ?? [] as m (m.id)}
							<div class="bubble" class:user={m.author === 'user'} class:agent={m.author !== 'user'}>
								<div class="bubble-head">
									<strong>{m.author === 'user' ? 'You' : 'PM agent'}</strong>
									<span class="muted">{fmtRelative(m.created_at)}</span>
								</div>
								<div class="bubble-content">{m.content}</div>
							</div>
						{/each}
					</div>

					{#if intakeError}<div class="err">{intakeError}</div>{/if}

					{#if conversation.status === 'finalized'}
						<p class="done-note">
							Conversation finalised. The PRD has been written — if the project isn't
							moving yet, click <strong>Finalize PRD</strong> to commit it.
						</p>
					{:else}
						<form class="reply-form" onsubmit={(e) => { e.preventDefault(); sendReply(); }}>
							<textarea
								bind:value={replyDraft}
								rows="3"
								placeholder="Your answer…"
								disabled={sending}
							></textarea>
							<div class="reply-actions">
								<button type="submit" disabled={sending || !replyDraft.trim()}>
									{sending ? 'Sending…' : 'Send'}
								</button>
								{#if intakeDone}
									<button
										type="button"
										class="primary"
										onclick={finalizePRD}
										disabled={finalizing}>
										{finalizing ? 'Writing PRD…' : '✓ Finalize PRD & start build'}
									</button>
									<span class="muted">PM has enough info to write the PRD. Keep chatting to refine, or finalize when ready.</span>
								{/if}
							</div>
						</form>
					{/if}
				{/if}
			</section>
		{:else if project.prd}
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
									<li class:active={isActive(story.status)}>
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
	.bar {
		position: relative;
		height: 18px;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 9px;
		overflow: hidden;
		margin-bottom: 0.75rem;
	}
	.bar-fill {
		height: 100%;
		background: linear-gradient(90deg, var(--accent), color-mix(in srgb, var(--accent) 60%, var(--ok)));
		transition: width 400ms ease-out;
	}
	.bar-fill.shipped {
		background: var(--ok);
	}
	.bar-label {
		position: absolute;
		inset: 0;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 0.7rem;
		font-weight: 600;
		color: var(--text);
		mix-blend-mode: difference;
		filter: invert(1);
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
	.prd {
		white-space: pre-wrap;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 4px;
		padding: 0.75rem;
		font-size: 0.85rem;
		overflow-x: auto;
	}
	.intake {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}
	.chat {
		display: flex;
		flex-direction: column;
		gap: 0.6rem;
		max-height: 520px;
		overflow-y: auto;
		padding: 0.5rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 6px;
	}
	.bubble {
		padding: 0.55rem 0.75rem;
		border-radius: 8px;
		max-width: 75%;
	}
	.bubble.user {
		align-self: flex-end;
		background: color-mix(in srgb, var(--accent) 18%, var(--bg));
		border: 1px solid color-mix(in srgb, var(--accent) 40%, var(--border));
	}
	.bubble.agent {
		align-self: flex-start;
		background: var(--bg);
		border: 1px solid var(--border);
	}
	.bubble-head {
		display: flex;
		gap: 0.5rem;
		font-size: 0.7rem;
		color: var(--muted);
		margin-bottom: 0.25rem;
	}
	.bubble-content {
		white-space: pre-wrap;
		font-size: 0.9rem;
		line-height: 1.4;
	}
	.reply-form {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}
	.reply-form textarea {
		padding: 0.55rem 0.75rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 6px;
		color: inherit;
		font: inherit;
		font-family: inherit;
		resize: vertical;
	}
	.reply-actions {
		display: flex;
		gap: 0.5rem;
		align-items: center;
		flex-wrap: wrap;
	}
	.reply-actions button {
		padding: 0.45rem 0.85rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 6px;
		cursor: pointer;
		color: inherit;
		font-weight: 500;
	}
	.reply-actions button.primary {
		background: var(--accent);
		color: white;
		border: none;
	}
	.reply-actions button:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.done-note {
		padding: 0.6rem 0.85rem;
		background: color-mix(in srgb, var(--ok) 12%, var(--bg-alt));
		border-left: 3px solid var(--ok);
		border-radius: 4px;
		font-size: 0.85rem;
	}
	.err {
		padding: 0.5rem 0.75rem;
		background: rgba(240, 80, 80, 0.15);
		border-left: 3px solid var(--err);
		border-radius: 4px;
		color: var(--err);
		font-size: 0.85rem;
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
		transition: border-color 200ms ease, box-shadow 200ms ease;
	}
	.stories li.active {
		border-color: var(--warn);
		box-shadow: 0 0 0 2px color-mix(in srgb, var(--warn) 35%, transparent);
		animation: pulse 1.8s ease-in-out infinite;
	}
	@keyframes pulse {
		0%, 100% { box-shadow: 0 0 0 2px color-mix(in srgb, var(--warn) 35%, transparent); }
		50%      { box-shadow: 0 0 0 4px color-mix(in srgb, var(--warn) 18%, transparent); }
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

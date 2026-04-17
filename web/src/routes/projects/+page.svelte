<script lang="ts">
	import { apiGet, apiPost, apiDelete } from '$lib/api';
	import { fmtRelative } from '$lib/format';
	import { createReconnectingWS, wsURL } from '$lib/ws';
	import ListScaffold from '$lib/ListScaffold.svelte';

	type Project = {
		id: string;
		name: string;
		idea: string;
		prd?: string;
		workdir?: string;
		status: string;
		created_at: string;
		updated_at: string;
	};

	let projects = $state<Project[]>([]);
	let loading = $state(true);
	let showForm = $state(false);
	let formError = $state('');
	let submitting = $state(false);

	// New project form
	let name = $state('');
	let idea = $state('');
	let workdir = $state('');
	let bmadOutputPath = $state('');
	let repoPath = $state('');

	async function load() {
		try {
			projects = (await apiGet<Project[]>('/api/v1/projects')) ?? [];
		} catch {
			/* banner */
		} finally {
			loading = false;
		}
	}

	// Any story.* or project.* event means a project row on this page
	// probably just flipped status — refresh the list rather than the
	// whole per-row payload (cheap, single query).
	function shouldRefresh(type: string): boolean {
		return (
			type.startsWith('story.') ||
			type.startsWith('project.') ||
			type === 'intake.finalized'
		);
	}

	$effect(() => {
		load();
		const i = setInterval(load, 15000);
		const ws = createReconnectingWS({
			url: wsURL('/ws'),
			onmessage: (msg) => {
				try {
					const evt = JSON.parse(msg.data) as { type?: string };
					if (!evt.type || !shouldRefresh(evt.type)) return;
					load();
				} catch {
					/* ignore non-JSON frames */
				}
			}
		});
		return () => {
			clearInterval(i);
			ws.close();
		};
	});

	async function createProject(ev: Event) {
		ev.preventDefault();
		formError = '';
		submitting = true;
		try {
			const p = (await apiPost('/api/v1/projects', {
				name,
				idea,
				workdir,
				bmad_output_path: bmadOutputPath,
				repo_path: repoPath
			})) as Project;
			// Send the user straight to the detail page — Phase 2 will start
			// the PM agent's Q&A from there.
			window.location.href = `/projects/${encodeURIComponent(p.id)}`;
		} catch (e) {
			formError = e instanceof Error ? e.message : String(e);
		} finally {
			submitting = false;
		}
	}

	async function removeProject(id: string, label: string) {
		if (!confirm(`Remove project "${label}"? Its epics, stories, and review history are deleted too.`))
			return;
		try {
			await apiDelete(`/api/v1/projects/${encodeURIComponent(id)}`);
			await load();
		} catch (e) {
			formError = e instanceof Error ? e.message : String(e);
		}
	}

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

<ListScaffold
	title="Projects"
	subtitle="Each project is an autonomous product build. Describe what you want, the BMAD agents take it from there — PM turns it into a PRD, Architect decomposes, Dev writes, Reviewer validates, until every acceptance criterion passes."
	{loading}
	isEmpty={projects.length === 0 && !showForm}
	emptyText="No projects yet. Describe what you want built."
>
	{#snippet controls()}
		<div class="toolbar">
			<button class="btn primary" onclick={() => (showForm = !showForm)}>
				{showForm ? 'Close' : '+ New project'}
			</button>
		</div>
	{/snippet}

	{#if showForm}
		<form class="create-form" onsubmit={createProject}>
			<label>
				What do you want to build?
				<textarea
					rows="3"
					placeholder="e.g. An app that helps writers draft, edit, and get AI-assisted feedback on their novels."
					bind:value={idea}
					required
				></textarea>
				<small>One clear sentence. The PM agent will ask you follow-ups in the next step.</small>
			</label>
			<label>
				Short name
				<input type="text" placeholder="auto-generated if empty" bind:value={name} />
			</label>
			<label>
				Working directory
				<input type="text" placeholder="/Users/me/projects/writers-app (optional for now)" bind:value={workdir} />
				<small>Where the Dev agent will commit code. Can be set later when the build actually starts.</small>
			</label>
			<label>
				Existing BMAD output path <span class="hint-pill">optional</span>
				<input type="text" placeholder="/Users/me/bmad-output/writers-app" bind:value={bmadOutputPath} />
				<small>If you've already run the BMAD method elsewhere (PRD, epics, stories), point at that directory and the Architect agent will skip decomposition and read the existing artefacts.</small>
			</label>
			<label>
				Existing repo <span class="hint-pill">optional</span>
				<input type="text" placeholder="/Users/me/projects/my-existing-app" bind:value={repoPath} />
				<small>Add BMAD to an existing codebase. Dev agents work inside this repo instead of scaffolding a fresh one.</small>
			</label>
			<button type="submit" disabled={submitting || !idea.trim()}>
				{submitting ? 'Creating…' : 'Create project'}
			</button>
			{#if formError}<div class="form-error">{formError}</div>{/if}
		</form>
	{/if}

	<table>
		<thead>
			<tr>
				<th>Project</th><th>Status</th><th>Updated</th><th></th>
			</tr>
		</thead>
		<tbody>
			{#each projects as p (p.id)}
				<tr>
					<td>
						<a class="pjrow" href="/projects/{p.id}">
							<strong>{p.name}</strong>
							<span class="muted">{p.idea}</span>
						</a>
					</td>
					<td><span class="badge" style="background:{statusColor(p.status)}">{p.status}</span></td>
					<td>{fmtRelative(p.updated_at)}</td>
					<td>
						<button class="row-del" onclick={() => removeProject(p.id, p.name)} title="Remove">✕</button>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</ListScaffold>

<style>
	.toolbar {
		margin: 1rem 0;
	}
	.btn.primary {
		padding: 0.5rem 1rem;
		background: var(--accent);
		color: white;
		border: none;
		border-radius: 6px;
		cursor: pointer;
		font-weight: 600;
	}
	.create-form {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		padding: 1rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 6px;
		margin-bottom: 1rem;
	}
	.create-form label {
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
		font-size: 0.85rem;
		color: var(--muted);
	}
	.create-form input,
	.create-form textarea {
		padding: 0.5rem 0.7rem;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 4px;
		color: inherit;
		font: inherit;
	}
	.create-form textarea {
		resize: vertical;
		font-family: inherit;
	}
	.create-form button {
		align-self: flex-start;
		padding: 0.5rem 1rem;
		background: var(--accent);
		color: white;
		border: none;
		border-radius: 4px;
		cursor: pointer;
		font-weight: 600;
	}
	.create-form button:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.create-form small {
		font-size: 0.75rem;
		color: var(--muted);
	}
	.hint-pill {
		display: inline-block;
		margin-left: 0.4rem;
		padding: 0 0.4rem;
		font-size: 0.65rem;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 999px;
		color: var(--muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}
	.form-error {
		padding: 0.5rem 0.75rem;
		background: rgba(240, 80, 80, 0.15);
		border-left: 3px solid var(--err);
		border-radius: 4px;
		color: var(--err);
		font-size: 0.85rem;
	}
	.pjrow {
		display: flex;
		flex-direction: column;
		gap: 0.15rem;
		color: inherit;
		text-decoration: none;
	}
	.pjrow:hover strong {
		color: var(--accent);
	}
	.muted {
		color: var(--muted);
		font-size: 0.85rem;
	}
	.row-del {
		padding: 0.2rem 0.45rem;
		background: transparent;
		color: var(--muted);
		border: 1px solid var(--border);
		border-radius: 3px;
		cursor: pointer;
		font-size: 0.8rem;
	}
	.row-del:hover {
		background: rgba(240, 80, 80, 0.15);
		color: var(--err);
		border-color: var(--err);
	}
</style>

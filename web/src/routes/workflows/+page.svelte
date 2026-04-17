<script lang="ts">
	import { fmtRelative } from '$lib/format';
	import { apiGet, apiPost, apiDelete } from '$lib/api';
	import type { Workflow } from '$lib/types';
	import ListScaffold from '$lib/ListScaffold.svelte';

	let workflows = $state<Workflow[]>([]);
	let loading = $state(true);

	let yaml = $state(`name: example
tasks:
  - name: review
    type: code-review
  - name: summarize
    type: summarize
    depends_on:
      - review
`);
	let editorOpen = $state(false);
	let formError = $state('');
	let submitting = $state(false);
	let firing = $state('');

	async function load() {
		try {
			workflows = (await apiGet<Workflow[]>('/api/v1/workflows')) ?? [];
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

	async function createWorkflow(ev: Event) {
		ev.preventDefault();
		formError = '';
		submitting = true;
		try {
			const r = await fetch('/api/v1/workflows', {
				method: 'POST',
				headers: { 'Content-Type': 'application/yaml' },
				body: yaml
			});
			const text = await r.text();
			if (!r.ok) {
				try {
					const json = JSON.parse(text);
					throw new Error(json?.error?.message ?? `${r.status}`);
				} catch (e) {
					if (e instanceof Error) throw e;
					throw new Error(text);
				}
			}
			editorOpen = false;
			await load();
		} catch (e) {
			formError = e instanceof Error ? e.message : String(e);
		} finally {
			submitting = false;
		}
	}

	async function fireWorkflow(name: string) {
		firing = name;
		formError = '';
		try {
			await apiPost(`/api/v1/workflows/${encodeURIComponent(name)}/runs`, {});
			await load();
		} catch (e) {
			formError = e instanceof Error ? e.message : String(e);
		} finally {
			firing = '';
		}
	}

	async function removeWorkflow(name: string) {
		if (!confirm(`Remove workflow "${name}"? In-flight runs are not cancelled.`)) return;
		try {
			await apiDelete(`/api/v1/workflows/${encodeURIComponent(name)}`);
			await load();
		} catch (e) {
			formError = e instanceof Error ? e.message : String(e);
		}
	}

	function badgeColor(status: string): string {
		if (status === 'running') return 'var(--warn)';
		if (status === 'completed') return 'var(--ok)';
		if (status === 'failed') return 'var(--err)';
		return 'var(--text-muted)';
	}
</script>

<ListScaffold
	title="Workflows"
	subtitle="Every workflow run recorded on this hive. Create a new workflow below, or fire an existing one."
	{loading}
	isEmpty={workflows.length === 0}
	emptyText="No workflows yet. Click 'New workflow' to create your first one, or run `hive run <file.yaml>`."
>
	<div class="toolbar">
		<a class="create-btn" href="/workflows/new">+ Visual builder</a>
		<button class="create-btn ghost" onclick={() => (editorOpen = !editorOpen)}>
			{editorOpen ? 'Close YAML' : 'Paste YAML'}
		</button>
	</div>

	{#if editorOpen}
		<form class="yaml-form" onsubmit={createWorkflow}>
			<label>
				Workflow YAML
				<textarea bind:value={yaml} rows="14" spellcheck="false"></textarea>
			</label>
			<button type="submit" disabled={submitting}>
				{submitting ? 'Creating…' : 'Create workflow'}
			</button>
		</form>
	{/if}

	{#if formError}<div class="form-error">{formError}</div>{/if}

	<table>
		<thead>
			<tr><th>Name</th><th>Status</th><th>ID</th><th>Started</th><th></th></tr>
		</thead>
		<tbody>
			{#each workflows as w (w.id)}
				<tr>
					<td><a class="wf-link" href="/workflows/{w.id}"><strong>{w.name}</strong></a></td>
					<td><span class="badge" style="background:{badgeColor(w.status)}">{w.status}</span></td>
					<td><a class="wf-link muted" href="/workflows/{w.id}"><code>{w.id.slice(-12)}</code></a></td>
					<td>{fmtRelative(w.created_at)}</td>
					<td class="actions">
						<button
							class="fire"
							onclick={() => fireWorkflow(w.name)}
							disabled={firing === w.name}
							title="Fire a manual run">
							{firing === w.name ? '…' : '▶'}
						</button>
						<button class="row-del" onclick={() => removeWorkflow(w.name)} title="Delete">✕</button>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</ListScaffold>

<style>
	.toolbar {
		margin-bottom: 1rem;
	}
	.toolbar {
		display: flex;
		gap: 0.5rem;
	}
	.create-btn {
		padding: 0.45rem 0.9rem;
		background: var(--accent);
		color: white;
		border: none;
		border-radius: 4px;
		cursor: pointer;
		font-weight: 600;
		text-decoration: none;
		display: inline-block;
	}
	.create-btn.ghost {
		background: transparent;
		color: var(--text);
		border: 1px solid var(--border);
	}
	.yaml-form {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		margin-bottom: 1rem;
		padding: 1rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 6px;
	}
	.yaml-form label {
		display: flex;
		flex-direction: column;
		gap: 0.35rem;
		font-size: 0.9rem;
		color: var(--muted);
	}
	.yaml-form textarea {
		padding: 0.55rem;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 4px;
		color: inherit;
		font-family: ui-monospace, monospace;
		font-size: 0.85rem;
		resize: vertical;
	}
	.yaml-form button {
		align-self: flex-start;
		padding: 0.5rem 1rem;
		background: var(--accent);
		color: white;
		border: none;
		border-radius: 4px;
		cursor: pointer;
		font-weight: 600;
	}
	.yaml-form button:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.form-error {
		padding: 0.5rem 0.75rem;
		background: rgba(240, 80, 80, 0.15);
		border-left: 3px solid var(--err);
		border-radius: 4px;
		color: var(--err);
		margin-bottom: 1rem;
		font-size: 0.85rem;
	}
	.actions {
		display: flex;
		gap: 0.3rem;
	}
	.fire {
		padding: 0.2rem 0.5rem;
		background: transparent;
		color: var(--muted);
		border: 1px solid var(--border);
		border-radius: 3px;
		cursor: pointer;
		font-size: 0.8rem;
	}
	.fire:hover {
		background: rgba(34, 197, 94, 0.15);
		color: var(--ok);
		border-color: var(--ok);
	}
	.fire:disabled {
		opacity: 0.5;
		cursor: not-allowed;
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
	.wf-link { color: inherit; text-decoration: none; }
	.wf-link:hover { color: var(--accent); }
	.muted { color: var(--muted); }
</style>

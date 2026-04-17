<script lang="ts">
	import { goto } from '$app/navigation';
	import {
		SvelteFlow,
		Controls,
		Background,
		MiniMap,
		type Node,
		type Edge,
		type Connection
	} from '@xyflow/svelte';
	import '@xyflow/svelte/dist/style.css';
	import yaml from 'js-yaml';

	type TaskNodeData = {
		label: string;
		type: string;
		input: string;
		condition: string;
		defaultBranch: boolean;
	};
	type TaskNode = Node<TaskNodeData, 'default'>;

	let nodes = $state<TaskNode[]>([
		{
			id: 't1',
			type: 'default',
			data: {
				label: 'review',
				type: 'code-review',
				input: '',
				condition: '',
				defaultBranch: false
			},
			position: { x: 150, y: 100 }
		}
	]);
	let edges = $state<Edge[]>([]);

	let name = $state('my-workflow');
	let allocation = $state<'' | 'capability-match' | 'market' | 'round-robin'>('');
	let concurrency = $state(0);

	let selectedId = $state<string | null>(null);
	let selected = $derived(() => nodes.find((n) => n.id === selectedId) ?? null);

	let generated = $state('');
	let submitError = $state('');
	let submitting = $state(false);

	function nextId(): string {
		return `t${Date.now().toString(36)}${nodes.length + 1}`;
	}

	function addTask() {
		nodes.push({
			id: nextId(),
			type: 'default',
			data: {
				label: `task_${nodes.length + 1}`,
				type: '',
				input: '',
				condition: '',
				defaultBranch: false
			},
			position: { x: 150 + nodes.length * 60, y: 150 + nodes.length * 80 }
		});
	}

	function deleteSelected() {
		if (!selectedId) return;
		const id = selectedId;
		nodes = nodes.filter((n) => n.id !== id);
		edges = edges.filter((e) => e.source !== id && e.target !== id);
		selectedId = null;
	}

	function handleConnect(c: Connection) {
		if (!c.source || !c.target || c.source === c.target) return;
		edges.push({ id: `e-${c.source}-${c.target}`, source: c.source, target: c.target });
	}

	function updateSelectedData(patch: Partial<TaskNodeData>) {
		if (!selectedId) return;
		const id = selectedId;
		nodes = nodes.map((n) => (n.id === id ? { ...n, data: { ...n.data, ...patch } } : n));
	}

	function buildYAML(): string {
		const tasks = nodes.map((n) => {
			const deps = edges
				.filter((e) => e.target === n.id)
				.map((e) => nodes.find((m) => m.id === e.source)?.data.label ?? '')
				.filter(Boolean);
			const task: Record<string, unknown> = {
				name: n.data.label,
				type: n.data.type
			};
			if (deps.length > 0) task.depends_on = deps;
			if (n.data.input) {
				try {
					task.input = JSON.parse(n.data.input);
				} catch {
					task.input = n.data.input;
				}
			}
			if (n.data.condition) task.condition = n.data.condition;
			if (n.data.defaultBranch) task.default = true;
			return task;
		});
		const cfg: Record<string, unknown> = { name, tasks };
		if (allocation) cfg.allocation = allocation;
		if (concurrency > 0) cfg.concurrency = concurrency;
		return yaml.dump(cfg, { noRefs: true, lineWidth: 120 });
	}

	function preview() {
		generated = buildYAML();
	}

	async function submit() {
		submitError = '';
		submitting = true;
		const body = buildYAML();
		try {
			const storedKey =
				typeof localStorage !== 'undefined' ? localStorage.getItem('hive.api_key') : null;
			const r = await fetch('/api/v1/workflows', {
				method: 'POST',
				body,
				headers: {
					'Content-Type': 'application/yaml',
					...(storedKey ? { Authorization: `Bearer ${storedKey}` } : {})
				}
			});
			if (!r.ok) {
				const text = await r.text();
				try {
					submitError = JSON.parse(text).error?.message ?? `${r.status}`;
				} catch {
					submitError = text || `${r.status}`;
				}
				return;
			}
			goto('/workflows');
		} catch (e) {
			submitError = e instanceof Error ? e.message : String(e);
		} finally {
			submitting = false;
		}
	}
</script>

<main>
	<header>
		<h1>New workflow</h1>
		<a class="back" href="/workflows">← all workflows</a>
	</header>

	<div class="toolbar">
		<label>
			Name
			<input type="text" bind:value={name} required />
		</label>
		<label>
			Allocation
			<select bind:value={allocation}>
				<option value="">capability-match (default)</option>
				<option value="market">market</option>
				<option value="round-robin">round-robin</option>
			</select>
		</label>
		<label>
			Concurrency
			<input type="number" min="0" bind:value={concurrency} title="0 = unlimited" />
		</label>
		<button class="btn" onclick={addTask}>+ Task</button>
		<button class="btn ghost" onclick={deleteSelected} disabled={!selectedId}>Delete selected</button>
		<button class="btn ghost" onclick={preview}>Preview YAML</button>
		<button class="btn primary" onclick={submit} disabled={submitting}>
			{submitting ? 'Creating…' : 'Create workflow'}
		</button>
	</div>

	{#if submitError}<div class="error">{submitError}</div>{/if}

	<div class="layout">
		<div class="canvas">
			<SvelteFlow
				bind:nodes
				bind:edges
				fitView
				onconnect={handleConnect}
				onnodeclick={({ node }) => (selectedId = node.id)}
				onpaneclick={() => (selectedId = null)}
			>
				<Background />
				<Controls />
				<MiniMap />
			</SvelteFlow>
		</div>

		<aside class="panel">
			{#if selected()}
				<h3>Task properties</h3>
				<label>
					Name
					<input
						type="text"
						value={selected()!.data.label}
						oninput={(e) => updateSelectedData({ label: (e.target as HTMLInputElement).value })}
					/>
				</label>
				<label>
					Type (capability)
					<input
						type="text"
						placeholder="e.g. code-review"
						value={selected()!.data.type}
						oninput={(e) => updateSelectedData({ type: (e.target as HTMLInputElement).value })}
					/>
				</label>
				<label>
					Input (JSON or string)
					<textarea
						rows="4"
						value={selected()!.data.input}
						oninput={(e) => updateSelectedData({ input: (e.target as HTMLTextAreaElement).value })}
					></textarea>
				</label>
				<label>
					Condition
					<input
						type="text"
						placeholder="upstream.review.score > 0.8"
						value={selected()!.data.condition}
						oninput={(e) => updateSelectedData({ condition: (e.target as HTMLInputElement).value })}
					/>
				</label>
				<label class="inline">
					<input
						type="checkbox"
						checked={selected()!.data.defaultBranch}
						onchange={(e) => updateSelectedData({ defaultBranch: (e.target as HTMLInputElement).checked })}
					/>
					<span>Default branch (runs when no sibling condition matched)</span>
				</label>
			{:else}
				<p class="hint">
					Click a node to edit properties. Drag from a node's handle to another node to create a
					dependency. + Task adds a new step.
				</p>
			{/if}

			{#if generated}
				<h3>Generated YAML</h3>
				<pre>{generated}</pre>
			{/if}
		</aside>
	</div>
</main>

<style>
	main {
		display: flex;
		flex-direction: column;
		gap: 1rem;
		height: calc(100vh - 4rem);
	}
	header {
		display: flex;
		justify-content: space-between;
		align-items: baseline;
	}
	h1 { margin: 0; }
	.back { color: var(--muted); text-decoration: none; font-size: 0.85rem; }
	.back:hover { color: var(--accent); }
	.toolbar {
		display: flex;
		gap: 0.5rem;
		align-items: end;
		flex-wrap: wrap;
	}
	.toolbar label {
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
		font-size: 0.8rem;
		color: var(--muted);
	}
	.toolbar input,
	.toolbar select {
		padding: 0.35rem 0.55rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 4px;
		color: inherit;
		font: inherit;
	}
	.btn {
		padding: 0.4rem 0.8rem;
		background: var(--accent);
		color: white;
		border: none;
		border-radius: 4px;
		cursor: pointer;
		font-weight: 500;
	}
	.btn.ghost {
		background: transparent;
		color: var(--text);
		border: 1px solid var(--border);
	}
	.btn.primary { margin-left: auto; }
	.btn:disabled { opacity: 0.5; cursor: not-allowed; }
	.error {
		padding: 0.5rem 0.75rem;
		background: rgba(240, 80, 80, 0.15);
		border-left: 3px solid var(--err);
		border-radius: 4px;
		color: var(--err);
		font-size: 0.85rem;
	}
	.layout {
		flex: 1;
		display: grid;
		grid-template-columns: 1fr 340px;
		gap: 1rem;
		min-height: 0;
	}
	.canvas {
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 6px;
		overflow: hidden;
	}
	.panel {
		padding: 1rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 6px;
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		overflow-y: auto;
	}
	.panel h3 { margin: 0; font-size: 0.95rem; }
	.panel label {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
		font-size: 0.8rem;
		color: var(--muted);
	}
	.panel label.inline {
		flex-direction: row;
		align-items: center;
	}
	.panel input,
	.panel textarea {
		padding: 0.35rem 0.55rem;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 4px;
		color: inherit;
		font: inherit;
		font-family: ui-monospace, monospace;
		font-size: 0.85rem;
	}
	.panel pre {
		margin: 0;
		padding: 0.6rem;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 4px;
		font-size: 0.75rem;
		overflow-x: auto;
		max-height: 280px;
		white-space: pre;
	}
	.panel .hint {
		color: var(--muted);
		font-style: italic;
		font-size: 0.85rem;
	}
</style>

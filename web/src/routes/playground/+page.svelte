<script lang="ts">
	import { apiGet, apiPost } from '$lib/api';
	import type { Agent } from '$lib/types';

	let agents = $state<Agent[]>([]);
	let selectedAgent = $state('');
	let taskType = $state('');
	let inputText = $state('{\n  "example": "payload"\n}');
	let result = $state<unknown>(null);
	let error = $state('');
	let running = $state(false);

	$effect(() => {
		apiGet<Agent[]>('/api/v1/agents').then((list) => {
			agents = list ?? [];
			if (agents.length > 0 && !selectedAgent) {
				selectedAgent = agents[0].name;
				taskType = taskTypesFor(agents[0])[0] ?? '';
			}
		});
	});

	function taskTypesFor(a: Agent): string[] {
		try {
			const parsed = JSON.parse(a.capabilities);
			return parsed.task_types ?? [];
		} catch {
			return [];
		}
	}

	async function invoke() {
		error = '';
		result = null;
		running = true;
		try {
			let input: unknown = inputText;
			if (inputText.trim().startsWith('{') || inputText.trim().startsWith('[')) {
				try {
					input = JSON.parse(inputText);
				} catch (e) {
					error = `Invalid JSON input: ${e instanceof Error ? e.message : String(e)}`;
					return;
				}
			}
			result = await apiPost(`/api/v1/agents/${encodeURIComponent(selectedAgent)}/invoke`, {
				type: taskType,
				input
			});
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
		} finally {
			running = false;
		}
	}

	let selectedAgentObj = $derived(agents.find((a) => a.name === selectedAgent));
	let availableTypes = $derived(selectedAgentObj ? taskTypesFor(selectedAgentObj) : []);
</script>

<h1>Playground</h1>
<p class="subtitle">Invoke a registered agent with an ad-hoc task. No workflow needed — handy for verifying connectivity and probing capabilities.</p>

{#if agents.length === 0}
	<div class="empty">No agents registered. Add one on the <a href="/agents">Agents page</a> first.</div>
{:else}
	<div class="grid">
		<label>
			Agent
			<select bind:value={selectedAgent}>
				{#each agents as a (a.id)}
					<option value={a.name}>{a.name} <span class="muted">({a.type} v{a.version ?? '1.0.0'})</span></option>
				{/each}
			</select>
		</label>

		<label>
			Task type
			<input list="task-types" bind:value={taskType} placeholder="e.g. code-review" />
			<datalist id="task-types">
				{#each availableTypes as t}<option value={t}></option>{/each}
			</datalist>
		</label>

		<label class="full">
			Input
			<textarea bind:value={inputText} rows="8" spellcheck="false"></textarea>
		</label>

		<button onclick={invoke} disabled={running || !selectedAgent || !taskType}>
			{running ? 'Invoking…' : 'Invoke agent'}
		</button>
	</div>

	{#if error}
		<div class="error">{error}</div>
	{/if}

	{#if result !== null}
		<h2>Result</h2>
		<pre class="result">{JSON.stringify(result, null, 2)}</pre>
	{/if}
{/if}

<style>
	h1 {
		margin-top: 0;
	}
	.subtitle {
		color: var(--muted);
		margin-bottom: 1.5rem;
	}
	.grid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 1rem;
		max-width: 700px;
	}
	.grid label {
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
		font-size: 0.85rem;
		color: var(--muted);
	}
	.grid .full {
		grid-column: 1 / -1;
	}
	.grid select,
	.grid input,
	.grid textarea {
		padding: 0.5rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 4px;
		color: inherit;
		font: inherit;
	}
	.grid textarea {
		font-family: var(--font-mono);
		font-size: 0.85rem;
	}
	.grid .muted {
		color: var(--muted);
	}
	button {
		grid-column: 1 / -1;
		padding: 0.6rem 1rem;
		background: var(--accent);
		color: white;
		border: none;
		border-radius: 4px;
		cursor: pointer;
		font-weight: 600;
	}
	button:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.empty {
		padding: 1rem;
		background: var(--bg-alt);
		border-radius: 4px;
		color: var(--muted);
	}
	.error {
		margin-top: 1rem;
		padding: 0.75rem 1rem;
		background: rgba(240, 80, 80, 0.15);
		border-left: 3px solid var(--err);
		border-radius: 4px;
		color: var(--err);
	}
	.result {
		margin-top: 0.5rem;
		padding: 1rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 4px;
		overflow-x: auto;
		font-size: 0.85rem;
	}
</style>

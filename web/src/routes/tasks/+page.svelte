<script lang="ts">
	import { apiGet } from '$lib/api';
	import { createReconnectingWS, wsURL } from '$lib/ws';
	import { fmtDuration, truncate } from '$lib/format';

	type Task = {
		id: string;
		workflow_id: string;
		type: string;
		status: string;
		agent_id: string;
		agent_name: string;
		created_at: string;
		duration_seconds?: number | null;
		result_summary?: string;
	};

	let tasks = $state<Task[]>([]);
	let loading = $state(true);

	async function loadTasks() {
		try {
			tasks = (await apiGet<Task[]>('/api/v1/tasks')) ?? [];
		} catch {
			/* banner shown by apiGet */
		} finally {
			loading = false;
		}
	}

	// Story 8.3 AC2: real-time updates. Keep the slow poll as a safety net
	// (agents-without-WS, missed frames) but drop the interval from 3s to 10s
	// now that task.* events trigger an immediate reload.
	$effect(() => {
		loadTasks();
		const interval = setInterval(loadTasks, 10000);
		const ws = createReconnectingWS({
			url: wsURL('/ws'),
			onmessage: (msg) => {
				try {
					const evt = JSON.parse(msg.data);
					if (typeof evt.type === 'string' && evt.type.startsWith('task.')) {
						loadTasks();
					}
				} catch {
					/* ignore non-JSON frames */
				}
			}
		});
		return () => {
			ws.close();
			clearInterval(interval);
		};
	});

	let grouped = $derived(() => {
		const byWf = new Map<string, Task[]>();
		for (const t of tasks) {
			const arr = byWf.get(t.workflow_id) ?? [];
			arr.push(t);
			byWf.set(t.workflow_id, arr);
		}
		return Array.from(byWf.entries()).map(([wf, ts]) => ({
			workflowId: wf,
			tasks: ts,
			counts: countByStatus(ts)
		}));
	});

	function countByStatus(ts: Task[]): Record<string, number> {
		const out: Record<string, number> = {};
		for (const t of ts) out[t.status] = (out[t.status] ?? 0) + 1;
		return out;
	}

	function statusColor(status: string): string {
		const colors: Record<string, string> = {
			pending: '#94a3b8',
			assigned: '#3b82f6',
			running: '#f59e0b',
			completed: '#22c55e',
			failed: '#ef4444'
		};
		return colors[status] ?? '#666';
	}
</script>

<main>
	<h1>Tasks</h1>

	{#if loading}
		<p class="empty">Loading…</p>
	{:else if tasks.length === 0}
		<p class="empty">No tasks yet. Run a workflow with <code>hive run</code>.</p>
	{:else}
		{#each grouped() as group (group.workflowId)}
			<section class="workflow">
				<header>
					<h2>Workflow <code>{group.workflowId}</code></h2>
					<div class="counts">
						{#each Object.entries(group.counts) as [status, n]}
							<span class="badge" style="background:{statusColor(status)}">{status} · {n}</span>
						{/each}
					</div>
				</header>
				<table>
					<thead>
						<tr>
							<th>ID</th>
							<th>Type</th>
							<th>Status</th>
							<th>Agent</th>
							<th>Duration</th>
							<th>Result</th>
							<th>Created</th>
						</tr>
					</thead>
					<tbody>
						{#each group.tasks as t (t.id)}
							<tr>
								<td><code>{t.id.slice(-8)}</code></td>
								<td>{t.type}</td>
								<td><span class="badge" style="background:{statusColor(t.status)}">{t.status}</span></td>
								<td>{t.agent_name || '—'}</td>
								<td>{fmtDuration(t.duration_seconds)}</td>
								<td class="result" title={t.result_summary ?? ''}>{truncate(t.result_summary ?? '', 60) || '—'}</td>
								<td>{t.created_at}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</section>
		{/each}
	{/if}
</main>

<style>
	main {
		font-family: system-ui, sans-serif;
	}
	.workflow {
		margin: 1.5rem 0;
		border: 1px solid #e5e7eb;
		border-radius: 8px;
		overflow: hidden;
	}
	.workflow header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		padding: 0.75rem 1rem;
		background: #f8fafc;
		border-bottom: 1px solid #e5e7eb;
	}
	.workflow h2 {
		margin: 0;
		font-size: 1rem;
		font-weight: 600;
	}
	.counts {
		display: flex;
		gap: 0.5rem;
	}
	.badge {
		display: inline-block;
		padding: 0.125rem 0.5rem;
		border-radius: 4px;
		color: white;
		font-size: 0.75rem;
		font-weight: 500;
	}
	table {
		width: 100%;
		border-collapse: collapse;
	}
	th,
	td {
		padding: 0.5rem 1rem;
		text-align: left;
		border-bottom: 1px solid #f1f5f9;
	}
	th {
		font-weight: 600;
		color: #64748b;
		background: #fafafa;
		font-size: 0.75rem;
		text-transform: uppercase;
	}
	.empty {
		color: #666;
		font-style: italic;
	}
	code {
		font-size: 0.75rem;
		background: #f3f4f6;
		padding: 1px 4px;
		border-radius: 3px;
	}
	.result {
		font-family: ui-monospace, monospace;
		font-size: 0.75rem;
		color: #475569;
		max-width: 320px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
</style>

<script lang="ts">
	type Task = {
		id: string;
		workflow_id: string;
		type: string;
		status: string;
		agent_id: string;
		created_at: string;
	};

	let tasks = $state<Task[]>([]);

	async function loadTasks() {
		try {
			const res = await fetch('/api/v1/events?type=task');
			const json = await res.json();
			tasks = json.data ?? [];
		} catch { /* API not ready */ }
	}

	$effect(() => {
		loadTasks();
		const interval = setInterval(loadTasks, 3000);
		return () => clearInterval(interval);
	});

	function statusBadge(status: string): string {
		const colors: Record<string, string> = {
			pending: '#94a3b8', assigned: '#3b82f6', running: '#f59e0b',
			completed: '#22c55e', failed: '#ef4444'
		};
		return colors[status] ?? '#666';
	}
</script>

<main>
	<h1>Tasks</h1>

	{#if tasks.length === 0}
		<p class="empty">No task events yet.</p>
	{:else}
		<table>
			<thead>
				<tr><th>ID</th><th>Type</th><th>Source</th><th>Time</th></tr>
			</thead>
			<tbody>
				{#each tasks as t}
					<tr>
						<td><code>{t.id}</code></td>
						<td>{t.type}</td>
						<td>{t.source}</td>
						<td>{t.created_at}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</main>

<style>
	main { font-family: system-ui, sans-serif; }
	table { width: 100%; border-collapse: collapse; margin-top: 1rem; }
	th, td { padding: 0.75rem; text-align: left; border-bottom: 1px solid #eee; }
	th { font-weight: 600; color: #666; }
	.empty { color: #666; font-style: italic; }
	code { font-size: 0.75rem; }
	a { color: #333; }
</style>

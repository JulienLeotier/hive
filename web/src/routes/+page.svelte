<script lang="ts">
	import { fmtRelative } from '$lib/format';

	type Metrics = {
		agents: { total: number; healthy: number; degraded: number; unavailable: number };
		tasks: Record<string, number>;
		workflows: Record<string, number>;
		circuit_breakers: { total: number; open: number };
		events: { last_minute: number; last_hour: number };
		avg_task_duration_seconds: number;
		timestamp: string;
	};

	type Event = { id: number; type: string; source: string; payload: string; created_at: string };

	let metrics = $state<Metrics | null>(null);
	let recentEvents = $state<Event[]>([]);
	let ws: WebSocket | null = $state(null);

	async function load() {
		try {
			const [m, e] = await Promise.all([
				fetch('/api/v1/metrics').then((r) => r.json()),
				fetch('/api/v1/events?limit=10').then((r) => r.json())
			]);
			metrics = m.data ?? null;
			recentEvents = e.data ?? [];
		} catch {
			/* api not ready */
		}
	}

	function connectWS() {
		const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
		ws = new WebSocket(`${proto}//${location.host}/ws`);
		ws.onmessage = (msg) => {
			try {
				const evt = JSON.parse(msg.data);
				recentEvents = [evt, ...recentEvents].slice(0, 10);
			} catch {
				/* ignore */
			}
		};
		ws.onclose = () => setTimeout(connectWS, 3000);
	}

	$effect(() => {
		load();
		connectWS();
		const interval = setInterval(load, 5000);
		return () => {
			ws?.close();
			clearInterval(interval);
		};
	});

	function taskCount(): number {
		if (!metrics?.tasks) return 0;
		return Object.values(metrics.tasks).reduce((a, b) => a + b, 0);
	}

	function runningTasks(): number {
		return (metrics?.tasks?.running ?? 0) + (metrics?.tasks?.assigned ?? 0);
	}

	function healthClass(): string {
		if (!metrics) return '';
		if (metrics.circuit_breakers.open > 0 || metrics.agents.unavailable > 0) return 'warn';
		if (metrics.agents.degraded > 0) return 'caution';
		return 'ok';
	}
</script>

<main>
	<h1>Hive</h1>
	<p class="subtitle">Universal AI agent orchestration</p>

	<div class="grid">
		<div class="card {healthClass()}">
			<div class="card-label">Fleet</div>
			<div class="card-value">{metrics?.agents.total ?? 0}</div>
			<div class="card-detail">
				<span class="dot ok"></span>{metrics?.agents.healthy ?? 0} healthy
				{#if metrics?.agents.degraded}
					<span class="dot caution"></span>{metrics.agents.degraded}
				{/if}
				{#if metrics?.agents.unavailable}
					<span class="dot warn"></span>{metrics.agents.unavailable}
				{/if}
			</div>
		</div>

		<div class="card">
			<div class="card-label">Tasks (running / total)</div>
			<div class="card-value">{runningTasks()} / {taskCount()}</div>
			<div class="card-detail">
				{metrics?.tasks?.pending ?? 0} pending · {metrics?.tasks?.failed ?? 0} failed
			</div>
		</div>

		<div class="card">
			<div class="card-label">Events / min</div>
			<div class="card-value">{metrics?.events?.last_minute ?? 0}</div>
			<div class="card-detail">{metrics?.events?.last_hour ?? 0} in the last hour</div>
		</div>

		<div class="card {metrics?.circuit_breakers.open ? 'warn' : ''}">
			<div class="card-label">Breakers open</div>
			<div class="card-value">{metrics?.circuit_breakers.open ?? 0}</div>
			<div class="card-detail">of {metrics?.circuit_breakers.total ?? 0} tracked</div>
		</div>

		<div class="card">
			<div class="card-label">Avg task duration</div>
			<div class="card-value">
				{(metrics?.avg_task_duration_seconds ?? 0).toFixed(1)}<span class="unit">s</span>
			</div>
			<div class="card-detail">Last 24h</div>
		</div>

		<div class="card">
			<div class="card-label">Workflows</div>
			<div class="card-value">
				{(metrics?.workflows?.running ?? 0) + (metrics?.workflows?.idle ?? 0)}
			</div>
			<div class="card-detail">
				{metrics?.workflows?.completed ?? 0} completed · {metrics?.workflows?.failed ?? 0} failed
			</div>
		</div>
	</div>

	<h2>Recent events</h2>
	{#if recentEvents.length === 0}
		<div class="empty">No events yet.</div>
	{:else}
		<table>
			<thead>
				<tr><th>Time</th><th>Type</th><th>Source</th><th>Payload</th></tr>
			</thead>
			<tbody>
				{#each recentEvents as e (e.id)}
					<tr>
						<td>{fmtRelative(e.created_at)}</td>
						<td><code>{e.type}</code></td>
						<td>{e.source}</td>
						<td class="payload">{e.payload}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</main>

<style>
	.subtitle {
		color: var(--text-muted);
		margin-top: 0;
	}
	.grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
		gap: 1rem;
		margin: 1.5rem 0;
	}
	.card {
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 1rem 1.25rem;
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}
	.card.warn {
		border-color: var(--err);
	}
	.card.caution {
		border-color: var(--warn);
	}
	.card-label {
		font-size: 0.7rem;
		color: var(--text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}
	.card-value {
		font-size: 1.75rem;
		font-weight: 600;
	}
	.unit {
		font-size: 1rem;
		color: var(--text-muted);
		margin-left: 2px;
	}
	.card-detail {
		font-size: 0.8rem;
		color: var(--text-muted);
	}
	.dot {
		display: inline-block;
		width: 8px;
		height: 8px;
		border-radius: 50%;
		margin-right: 4px;
		margin-left: 8px;
	}
	.dot.ok {
		background: var(--ok);
		margin-left: 0;
	}
	.dot.caution {
		background: var(--warn);
	}
	.dot.warn {
		background: var(--err);
	}
	.payload {
		font-family: monospace;
		font-size: 0.75rem;
		color: var(--text-muted);
		max-width: 600px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
</style>

<script lang="ts">
	let agentCount = $state(0);
	let taskCount = $state(0);
	let eventCount = $state(0);

	async function loadMetrics() {
		try {
			const res = await fetch('/api/v1/metrics');
			const json = await res.json();
			if (json.data) {
				agentCount = json.data.agents?.total ?? 0;
			}
		} catch {
			// API not available yet
		}
	}

	$effect(() => {
		loadMetrics();
		const interval = setInterval(loadMetrics, 5000);
		return () => clearInterval(interval);
	});
</script>

<main>
	<h1>Hive Dashboard</h1>
	<p class="subtitle">Universal AI Agent Orchestration</p>

	<div class="stats">
		<div class="stat">
			<span class="value">{agentCount}</span>
			<span class="label">Agents</span>
		</div>
		<div class="stat">
			<span class="value">{taskCount}</span>
			<span class="label">Tasks</span>
		</div>
		<div class="stat">
			<span class="value">{eventCount}</span>
			<span class="label">Events</span>
		</div>
	</div>

	<nav>
		<a href="/agents">Agents</a>
		<a href="/tasks">Tasks</a>
		<a href="/events">Events</a>
	</nav>
</main>

<style>
	main {
		max-width: 800px;
		margin: 0 auto;
		padding: 2rem;
		font-family: system-ui, -apple-system, sans-serif;
	}
	h1 { margin-bottom: 0.25rem; }
	.subtitle { color: #666; margin-top: 0; }
	.stats {
		display: flex;
		gap: 2rem;
		margin: 2rem 0;
	}
	.stat {
		background: #f5f5f5;
		border-radius: 8px;
		padding: 1.5rem 2rem;
		text-align: center;
		flex: 1;
	}
	.value {
		display: block;
		font-size: 2rem;
		font-weight: bold;
	}
	.label {
		color: #666;
		font-size: 0.875rem;
	}
	nav {
		display: flex;
		gap: 1rem;
	}
	nav a {
		background: #333;
		color: white;
		padding: 0.5rem 1.5rem;
		border-radius: 4px;
		text-decoration: none;
	}
	nav a:hover { background: #555; }
</style>

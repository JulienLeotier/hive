<script lang="ts">
	import { apiGet } from '$lib/api';

	let tenants = $state<string[]>([]);
	let loading = $state(true);

	async function load() {
		try {
			tenants = (await apiGet<string[]>('/api/v1/tenants')) ?? [];
		} catch {
			/* banner shown by apiGet */
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		load();
	});
</script>

<main>
	<h1>Tenants</h1>
	<p class="subtitle">Each tenant scopes its own agents, workflows, tasks, events, and knowledge.</p>

	{#if loading}
		<div class="empty">Loading…</div>
	{:else if tenants.length === 0}
		<div class="empty">No tenants. Create one with <code>hive tenant create &lt;id&gt;</code>.</div>
	{:else}
		<div class="tenant-grid">
			{#each tenants as t}
				<div class="tenant-card">
					<div class="icon">⬡</div>
					<div class="id">{t}</div>
				</div>
			{/each}
		</div>
	{/if}
</main>

<style>
	.subtitle {
		color: var(--text-muted);
		margin-top: 0;
	}
	.tenant-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
		gap: 0.75rem;
		margin: 1rem 0;
	}
	.tenant-card {
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 1rem;
		text-align: center;
	}
	.icon {
		font-size: 2rem;
		color: var(--accent);
	}
	.id {
		margin-top: 0.5rem;
		font-weight: 600;
	}
</style>

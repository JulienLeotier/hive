<script lang="ts">
	import { apiGet } from '$lib/api';
	import ListScaffold from '$lib/ListScaffold.svelte';

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

<ListScaffold
	title="Tenants"
	subtitle="Each tenant scopes its own agents, workflows, tasks, events, and knowledge."
	{loading}
	isEmpty={tenants.length === 0}
	emptyText="No tenants. Create one with `hive tenant create <id>`."
>
	<div class="tenant-grid">
		{#each tenants as t}
			<div class="tenant-card">
				<div class="icon">⬡</div>
				<div class="id">{t}</div>
			</div>
		{/each}
	</div>
</ListScaffold>

<style>
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

<script lang="ts">
	import { fmtRelative } from '$lib/format';
	import { apiGet } from '$lib/api';
	import type { FederationLink } from '$lib/types';

	let links = $state<FederationLink[]>([]);
	let loading = $state(true);

	async function load() {
		try {
			links = (await apiGet<FederationLink[]>('/api/v1/federation')) ?? [];
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

	function parseCaps(s: string): string[] {
		try {
			return JSON.parse(s ?? '[]');
		} catch {
			return [];
		}
	}
</script>

<main>
	<h1>Federation</h1>
	<p class="subtitle">Links to peer Hive deployments. Capabilities exchanged, task data stays local.</p>

	{#if loading}
		<div class="empty">Loading…</div>
	{:else if links.length === 0}
		<div class="empty">
			No federation links. Connect one with <code>hive federation connect &lt;name&gt; &lt;url&gt;</code>.
		</div>
	{:else}
		<table>
			<thead>
				<tr><th>Name</th><th>URL</th><th>Status</th><th>Shared capabilities</th><th>Last heartbeat</th></tr>
			</thead>
			<tbody>
				{#each links as l (l.name)}
					<tr>
						<td><strong>{l.name}</strong></td>
						<td><code>{l.url}</code></td>
						<td>
							<span
								class="badge"
								style="background:{l.status === 'active'
									? 'var(--ok)'
									: l.status === 'degraded'
										? 'var(--warn)'
										: 'var(--err)'}">{l.status}</span
							>
						</td>
						<td>
							{#each parseCaps(l.shared_caps) as cap}
								<span class="cap">{cap}</span>
							{/each}
							{#if parseCaps(l.shared_caps).length === 0}
								<span class="muted">all</span>
							{/if}
						</td>
						<td>{l.last_heartbeat ? fmtRelative(l.last_heartbeat) : '—'}</td>
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
	.cap {
		display: inline-block;
		padding: 2px 8px;
		border: 1px solid var(--border);
		border-radius: 4px;
		font-size: 0.75rem;
		margin-right: 4px;
	}
	.muted {
		color: var(--text-muted);
		font-style: italic;
	}
</style>

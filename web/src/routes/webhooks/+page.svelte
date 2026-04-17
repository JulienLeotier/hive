<script lang="ts">
	import { apiGet, apiPost, apiDelete } from '$lib/api';
	import ListScaffold from '$lib/ListScaffold.svelte';

	type Webhook = {
		id: string;
		name: string;
		url: string;
		type: string;
		event_filter: string;
		enabled: boolean;
	};
	type Delivery = {
		id: number;
		webhook_name: string;
		event_type: string;
		attempt: number;
		status_code: number;
		error?: string;
		created_at: string;
	};

	let webhooks = $state<Webhook[]>([]);
	let loading = $state(true);
	let expanded = $state<string | null>(null);
	let deliveries = $state<Record<string, Delivery[]>>({});

	let newName = $state('');
	let newURL = $state('');
	let newType = $state('generic');
	let newFilter = $state('');
	let formError = $state('');
	let submitting = $state(false);

	async function load() {
		try {
			webhooks = (await apiGet<Webhook[]>('/api/v1/webhooks')) ?? [];
		} catch {
			/* banner shown by apiGet */
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		load();
	});

	async function createWebhook(ev: Event) {
		ev.preventDefault();
		formError = '';
		submitting = true;
		try {
			await apiPost('/api/v1/webhooks', {
				name: newName,
				url: newURL,
				type: newType,
				event_filter: newFilter
			});
			newName = '';
			newURL = '';
			newFilter = '';
			await load();
		} catch (e) {
			formError = e instanceof Error ? e.message : String(e);
		} finally {
			submitting = false;
		}
	}

	async function removeWebhook(name: string) {
		if (!confirm(`Remove webhook "${name}"?`)) return;
		try {
			await apiDelete(`/api/v1/webhooks/${encodeURIComponent(name)}`);
			await load();
		} catch (e) {
			formError = e instanceof Error ? e.message : String(e);
		}
	}

	async function toggleHistory(name: string) {
		if (expanded === name) {
			expanded = null;
			return;
		}
		expanded = name;
		try {
			const list = await apiGet<Delivery[]>(
				`/api/v1/webhooks/${encodeURIComponent(name)}/deliveries?limit=50`
			);
			deliveries = { ...deliveries, [name]: list ?? [] };
		} catch (e) {
			formError = e instanceof Error ? e.message : String(e);
		}
	}

	function deliveryColor(d: Delivery): string {
		if (d.status_code >= 200 && d.status_code < 300) return 'var(--ok)';
		if (d.status_code === 0) return 'var(--err)';
		return 'var(--warn)';
	}
</script>

<ListScaffold
	title="Webhooks"
	subtitle="Outbound integrations fired from events. Slack / GitHub / generic. URLs are encrypted at rest when HIVE_MASTER_KEY is set."
	{loading}
	isEmpty={webhooks.length === 0}
	emptyText="No webhooks configured. Create one with the form below."
>
	<form class="create-form" onsubmit={createWebhook}>
		<input placeholder="name" bind:value={newName} required />
		<select bind:value={newType}>
			<option value="generic">generic</option>
			<option value="slack">slack</option>
			<option value="github">github</option>
		</select>
		<input placeholder="https://hooks.example.com/…" bind:value={newURL} required />
		<input placeholder="event filter (optional, e.g. task.failed)" bind:value={newFilter} />
		<button type="submit" disabled={submitting}>{submitting ? '…' : 'Add'}</button>
	</form>
	{#if formError}<div class="form-error">{formError}</div>{/if}
	<table>
		<thead>
			<tr>
				<th>Name</th><th>Type</th><th>URL</th><th>Filter</th><th>Enabled</th><th></th>
			</tr>
		</thead>
		<tbody>
			{#each webhooks as w (w.id)}
				<tr>
					<td>
						<button class="expand" onclick={() => toggleHistory(w.name)} title="Show delivery history">
							{expanded === w.name ? '▾' : '▸'}
						</button>
						<strong>{w.name}</strong>
					</td>
					<td><code>{w.type}</code></td>
					<td class="url"><code>{w.url}</code></td>
					<td><code>{w.event_filter || '—'}</code></td>
					<td>{w.enabled ? '✓' : '✗'}</td>
					<td><button class="row-del" onclick={() => removeWebhook(w.name)} title="Remove webhook">✕</button></td>
				</tr>
				{#if expanded === w.name}
					<tr class="history-row">
						<td colspan="6">
							{#if !deliveries[w.name] || deliveries[w.name].length === 0}
								<p class="empty-history">No deliveries yet.</p>
							{:else}
								<table class="history">
									<thead>
										<tr><th>When</th><th>Event</th><th>Attempt</th><th>Status</th><th>Error</th></tr>
									</thead>
									<tbody>
										{#each deliveries[w.name] as d (d.id)}
											<tr>
												<td><code class="ts">{d.created_at}</code></td>
												<td><code>{d.event_type}</code></td>
												<td>#{d.attempt}</td>
												<td>
													<span class="status-dot" style="background:{deliveryColor(d)}"></span>
													{d.status_code || '—'}
												</td>
												<td class="err">{d.error ?? ''}</td>
											</tr>
										{/each}
									</tbody>
								</table>
							{/if}
						</td>
					</tr>
				{/if}
			{/each}
		</tbody>
	</table>
</ListScaffold>

<style>
	.create-form {
		display: grid;
		grid-template-columns: 1fr 140px 2fr 2fr auto;
		gap: 0.5rem;
		margin-bottom: 1rem;
		align-items: center;
	}
	.create-form input,
	.create-form select {
		padding: 0.4rem 0.6rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 4px;
		color: inherit;
		font: inherit;
	}
	.create-form button {
		padding: 0.4rem 1rem;
		background: var(--accent);
		color: white;
		border: none;
		border-radius: 4px;
		cursor: pointer;
		font-weight: 600;
	}
	.create-form button:disabled {
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
	td.url {
		max-width: 300px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.expand {
		background: transparent;
		border: none;
		color: var(--muted);
		font-size: 0.75rem;
		cursor: pointer;
		padding: 0 0.3rem 0 0;
	}
	.expand:hover { color: var(--accent); }
	.history-row td {
		padding: 0.5rem 1rem 1rem 1rem;
		background: var(--bg-alt);
	}
	table.history {
		width: 100%;
		font-size: 0.8rem;
	}
	table.history th,
	table.history td {
		padding: 0.25rem 0.5rem;
		text-align: left;
	}
	.status-dot {
		display: inline-block;
		width: 8px;
		height: 8px;
		border-radius: 50%;
		margin-right: 0.4rem;
	}
	.ts { color: var(--muted); font-size: 0.75rem; }
	.err { color: var(--err); font-size: 0.75rem; }
	.empty-history { color: var(--muted); font-style: italic; margin: 0.5rem 0; }
</style>

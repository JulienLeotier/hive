<script lang="ts">
	import { fmtRelative } from '$lib/format';
	import { apiGet } from '$lib/api';
	import type { AuditEntry } from '$lib/types';

	let entries = $state<AuditEntry[]>([]);
	let actorFilter = $state('');
	let actionFilter = $state('');
	let resourceFilter = $state('');
	let search = $state('');
	let sinceHours = $state(24);
	let loading = $state(true);

	async function load() {
		try {
			entries = (await apiGet<AuditEntry[]>('/api/v1/audit')) ?? [];
		} catch {
			/* banner shown by apiGet */
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		load();
		const i = setInterval(load, 10000);
		return () => clearInterval(i);
	});

	let filtered = $derived(() => {
		const cutoff = sinceHours > 0 ? Date.now() - sinceHours * 3600 * 1000 : 0;
		const needle = search.trim().toLowerCase();
		return entries.filter((e) => {
			if (actorFilter && !e.actor.toLowerCase().includes(actorFilter.toLowerCase())) return false;
			if (actionFilter && !e.action.toLowerCase().includes(actionFilter.toLowerCase())) return false;
			if (resourceFilter && !e.resource.toLowerCase().includes(resourceFilter.toLowerCase())) return false;
			if (cutoff > 0) {
				const t = new Date(e.created_at).getTime();
				if (!isNaN(t) && t < cutoff) return false;
			}
			if (needle) {
				const hay = [e.actor, e.action, e.resource, e.detail ?? '']
					.join(' ')
					.toLowerCase();
				if (!hay.includes(needle)) return false;
			}
			return true;
		});
	});

	function clearFilters() {
		actorFilter = '';
		actionFilter = '';
		resourceFilter = '';
		search = '';
		sinceHours = 24;
	}

	let hasFilters = $derived(
		actorFilter !== '' || actionFilter !== '' || resourceFilter !== '' || search !== '' || sinceHours !== 24
	);
</script>

<svelte:head><title>Audit · Hive</title></svelte:head>

<h1>Audit log</h1>
<p class="sub">
	Journal des actions sensibles : créations, suppressions, changements de config.
	Filtre par acteur, action, ressource, ou texte libre dans le détail.
</p>

<div class="filters">
	<div class="filter-row">
		<input type="text" placeholder="Acteur" bind:value={actorFilter} />
		<input type="text" placeholder="Action" bind:value={actionFilter} />
		<input type="text" placeholder="Ressource" bind:value={resourceFilter} />
	</div>
	<div class="filter-row">
		<select bind:value={sinceHours} title="Fenêtre temporelle">
			<option value={1}>dernière heure</option>
			<option value={24}>24 dernières heures</option>
			<option value={168}>7 derniers jours</option>
			<option value={720}>30 derniers jours</option>
			<option value={0}>tout</option>
		</select>
		<input type="text" class="search" placeholder="Recherche libre…" bind:value={search} />
	</div>
	<div class="filter-actions">
		{#if hasFilters}
			<button class="btn ghost" onclick={clearFilters}>Effacer filtres</button>
		{/if}
		<span class="count">
			<strong>{filtered().length}</strong> / {entries.length} entrée{entries.length > 1 ? 's' : ''}
		</span>
	</div>
</div>

{#if loading && entries.length === 0}
	<div class="empty"><span class="empty-icon">⏳</span>Chargement…</div>
{:else if filtered().length === 0}
	<div class="empty">
		<span class="empty-icon">◌</span>
		{hasFilters ? 'Aucune entrée ne matche ces filtres.' : 'Aucune action auditée pour le moment.'}
	</div>
{:else}
	<!-- Desktop: table. Mobile: cards (bascule via CSS). -->
	<div class="table-scroll desktop-only">
		<table>
			<thead>
				<tr>
					<th>Quand</th><th>Acteur</th><th>Action</th><th>Ressource</th><th>Détail</th>
				</tr>
			</thead>
			<tbody>
				{#each filtered() as e (e.id)}
					<tr>
						<td class="when">{fmtRelative(e.created_at)}</td>
						<td><strong>{e.actor}</strong></td>
						<td><code>{e.action}</code></td>
						<td class="resource">{e.resource}</td>
						<td class="detail">{e.detail}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>

	<ul class="audit-cards mobile-only">
		{#each filtered() as e (e.id)}
			<li>
				<div class="card-head">
					<code class="action">{e.action}</code>
					<span class="when">{fmtRelative(e.created_at)}</span>
				</div>
				<div class="card-meta">
					<span><span class="k">Acteur</span> <strong>{e.actor}</strong></span>
					<span><span class="k">Ressource</span> {e.resource}</span>
				</div>
				{#if e.detail}
					<div class="card-detail">{e.detail}</div>
				{/if}
			</li>
		{/each}
	</ul>
{/if}

<style>
	.sub {
		color: var(--text-muted);
		font-size: 0.85rem;
		margin: 0 0 1.25rem;
	}

	/* ===== Filters ===== */
	.filters {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
		margin-bottom: 1.25rem;
	}
	.filter-row {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 0.5rem;
	}
	.filter-row:nth-child(2) {
		grid-template-columns: auto 1fr;
	}
	.filter-row input,
	.filter-row select {
		padding: 0.55rem 0.75rem;
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 6px;
		color: inherit;
		font: inherit;
		font-size: 0.85rem;
	}
	.filter-row input:focus,
	.filter-row select:focus {
		outline: none;
		border-color: var(--accent);
	}
	.filter-actions {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}
	.btn {
		padding: 0.45rem 0.85rem;
		border-radius: 6px;
		font-size: 0.82rem;
		font-weight: 600;
		cursor: pointer;
		border: 1px solid var(--border);
		background: transparent;
		color: inherit;
	}
	.btn.ghost:hover { color: var(--err); border-color: var(--err); }
	.count {
		margin-left: auto;
		font-size: 0.8rem;
		color: var(--text-muted);
	}
	.count strong {
		color: var(--text);
		font-variant-numeric: tabular-nums;
	}

	/* ===== Table (desktop) ===== */
	.when {
		font-size: 0.78rem;
		color: var(--text-muted);
		white-space: nowrap;
		font-variant-numeric: tabular-nums;
	}
	.resource {
		font-family: ui-monospace, monospace;
		font-size: 0.82rem;
	}
	.detail {
		color: var(--text-muted);
		font-size: 0.82rem;
		max-width: 500px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	/* ===== Cards (mobile) ===== */
	.audit-cards {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}
	.audit-cards li {
		padding: 0.75rem 0.9rem;
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 8px;
	}
	.card-head {
		display: flex;
		justify-content: space-between;
		align-items: baseline;
		gap: 0.5rem;
		margin-bottom: 0.4rem;
	}
	.card-head .action {
		font-family: ui-monospace, monospace;
		font-size: 0.82rem;
		font-weight: 600;
		color: var(--accent);
		background: var(--bg-hover);
		padding: 2px 8px;
		border-radius: 4px;
	}
	.card-head .when {
		font-size: 0.72rem;
		color: var(--text-muted);
	}
	.card-meta {
		display: flex;
		flex-wrap: wrap;
		gap: 0.3rem 1rem;
		font-size: 0.82rem;
		margin-bottom: 0.35rem;
	}
	.card-meta .k {
		display: inline-block;
		font-size: 0.68rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--text-muted);
		margin-right: 0.25rem;
	}
	.card-detail {
		font-size: 0.8rem;
		color: var(--text-muted);
		padding-top: 0.4rem;
		border-top: 1px dashed var(--border);
		word-break: break-word;
	}

	/* ===== Empty ===== */
	.empty {
		padding: 3rem 1rem;
		text-align: center;
		color: var(--text-muted);
		background: var(--bg-panel);
		border: 1px dashed var(--border);
		border-radius: 8px;
	}
	.empty-icon {
		display: block;
		font-size: 2rem;
		margin-bottom: 0.5rem;
		opacity: 0.5;
	}

	/* ===== Responsive toggle ===== */
	.desktop-only { display: block; }
	.mobile-only { display: none; }
	@media (max-width: 767px) {
		.desktop-only { display: none; }
		.mobile-only { display: flex; flex-direction: column; }
		.filter-row { grid-template-columns: 1fr; }
		.filter-row:nth-child(2) { grid-template-columns: 1fr; }
	}
</style>

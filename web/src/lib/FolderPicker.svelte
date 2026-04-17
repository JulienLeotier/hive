<script lang="ts">
	import { apiGet } from '$lib/api';

	type FsEntry = { name: string; path: string; is_dir: boolean };
	type FsList = { path: string; parent?: string; home: string; entries: FsEntry[] };

	let {
		value = $bindable(''),
		placeholder = '/Users/moi/projets/mon-app',
		label = 'Dossier'
	}: {
		value?: string;
		placeholder?: string;
		label?: string;
	} = $props();

	let open = $state(false);
	let listing = $state<FsList | null>(null);
	let loading = $state(false);
	let error = $state('');

	async function show() {
		open = true;
		error = '';
		await navigate(value.trim() || ''); // '' → backend returns home
	}

	async function navigate(path: string) {
		loading = true;
		try {
			const qs = path ? `?path=${encodeURIComponent(path)}` : '';
			listing = await apiGet<FsList>(`/api/v1/fs/list${qs}`);
			error = '';
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
		} finally {
			loading = false;
		}
	}

	function pick(entry: FsEntry) {
		if (entry.is_dir) navigate(entry.path);
	}

	function choose(path: string) {
		value = path;
		open = false;
	}

	function cancel() {
		open = false;
	}

	// Breadcrumb : segments cliquables du path courant.
	let crumbs = $derived.by(() => {
		if (!listing) return [] as { label: string; path: string }[];
		const parts = listing.path.split('/').filter(Boolean);
		const out: { label: string; path: string }[] = [{ label: '/', path: '/' }];
		let acc = '';
		for (const p of parts) {
			acc += '/' + p;
			out.push({ label: p, path: acc });
		}
		return out;
	});
</script>

<div class="wrap">
	<input type="text" {placeholder} bind:value />
	<button type="button" class="browse" onclick={show} title="Parcourir">📂</button>
</div>

{#if open}
	<div class="backdrop" role="dialog" aria-label={label}>
		<div class="modal">
			<header>
				<h3>{label}</h3>
				<button type="button" class="close" onclick={cancel} aria-label="Fermer">✕</button>
			</header>

			<div class="crumbs">
				{#each crumbs as c, i (c.path)}
					<button type="button" class="crumb" onclick={() => navigate(c.path)}>
						{i === 0 ? '/' : c.label}
					</button>
					{#if i < crumbs.length - 1}<span class="sep">/</span>{/if}
				{/each}
			</div>

			{#if error}
				<div class="err">{error}</div>
			{/if}

			<div class="body">
				{#if loading}
					<p class="empty">Chargement…</p>
				{:else if listing}
					{@const cur = listing}
					<button
						type="button"
						class="quick"
						onclick={() => navigate(cur.home)}
						title="Retour au dossier personnel">🏠 Home</button>
					{#if cur.parent}
						<button
							type="button"
							class="quick"
							onclick={() => navigate(cur.parent!)}>⬆ Dossier parent</button>
					{/if}
					<ul>
						{#each listing.entries as e (e.path)}
							<li>
								<button
									type="button"
									class="entry"
									onclick={() => pick(e)}
									title={e.path}>
									<span class="icon">📁</span>
									<span class="name">{e.name}</span>
								</button>
							</li>
						{/each}
						{#if listing.entries.length === 0}
							<li class="empty">Dossier vide</li>
						{/if}
					</ul>
				{/if}
			</div>

			<footer>
				<code class="cur">{listing?.path ?? ''}</code>
				<div class="foot-actions">
					<button type="button" onclick={cancel}>Annuler</button>
					<button type="button"
						class="primary"
						onclick={() => listing && choose(listing.path)}
						disabled={!listing}>
						Choisir ce dossier
					</button>
				</div>
			</footer>
		</div>
	</div>
{/if}

<style>
	.wrap { display: flex; gap: 0.3rem; }
	.wrap input { flex: 1; padding: 0.5rem 0.7rem; background: var(--bg); border: 1px solid var(--border); border-radius: 4px; color: inherit; font: inherit; }
	.browse { padding: 0 0.7rem; background: var(--bg-alt); border: 1px solid var(--border); border-radius: 4px; cursor: pointer; font-size: 1rem; }
	.browse:hover { border-color: var(--accent); }

	.backdrop { position: fixed; inset: 0; background: rgba(0, 0, 0, 0.55); display: flex; align-items: center; justify-content: center; z-index: 2000; }
	.modal {
		display: flex; flex-direction: column;
		width: min(640px, 94vw); max-height: 80vh;
		background: var(--bg-panel); border: 1px solid var(--border);
		border-radius: 8px; overflow: hidden;
		box-shadow: 0 16px 48px rgba(0, 0, 0, 0.35);
	}
	header { display: flex; justify-content: space-between; align-items: center; padding: 0.6rem 1rem; background: var(--bg-alt); border-bottom: 1px solid var(--border); }
	header h3 { margin: 0; font-size: 0.95rem; }
	.close { background: none; border: none; color: var(--muted); cursor: pointer; font-size: 1rem; }
	.close:hover { color: var(--err); }

	.crumbs { display: flex; flex-wrap: wrap; align-items: center; padding: 0.5rem 1rem; gap: 0.2rem; font-family: ui-monospace, monospace; font-size: 0.8rem; border-bottom: 1px solid var(--border); background: var(--bg); }
	.crumb { background: none; border: none; color: var(--accent); cursor: pointer; padding: 0 0.2rem; font: inherit; }
	.crumb:hover { text-decoration: underline; }
	.sep { color: var(--muted); }

	.body { overflow-y: auto; flex: 1; padding: 0.5rem 1rem; }
	.body ul { list-style: none; padding: 0; margin: 0.25rem 0 0; display: flex; flex-direction: column; gap: 0.1rem; }
	.entry { display: flex; gap: 0.5rem; align-items: center; width: 100%; text-align: left; padding: 0.35rem 0.5rem; background: transparent; border: none; border-radius: 3px; color: inherit; font: inherit; cursor: pointer; }
	.entry:hover { background: var(--bg-alt); }
	.icon { font-size: 0.9rem; }
	.name { font-family: ui-monospace, monospace; font-size: 0.85rem; }
	.quick { display: block; width: 100%; text-align: left; padding: 0.3rem 0.5rem; background: transparent; border: 1px dashed var(--border); border-radius: 3px; color: var(--muted); cursor: pointer; font: inherit; font-size: 0.8rem; margin-bottom: 0.15rem; }
	.quick:hover { color: var(--accent); border-color: var(--accent); }
	.empty { color: var(--muted); font-style: italic; padding: 0.5rem; }

	footer { display: flex; justify-content: space-between; align-items: center; gap: 0.5rem; padding: 0.5rem 1rem; background: var(--bg-alt); border-top: 1px solid var(--border); }
	.cur { font-family: ui-monospace, monospace; font-size: 0.75rem; color: var(--muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 320px; }
	.foot-actions { display: flex; gap: 0.4rem; }
	.foot-actions button { padding: 0.35rem 0.8rem; border: 1px solid var(--border); background: var(--bg); border-radius: 4px; cursor: pointer; color: inherit; font: inherit; font-size: 0.85rem; }
	.foot-actions button.primary { background: var(--accent); color: white; border: none; font-weight: 600; }
	.foot-actions button.primary:disabled { opacity: 0.5; cursor: not-allowed; }

	.err { margin: 0.5rem 1rem; padding: 0.4rem 0.6rem; background: rgba(240,80,80,0.15); border-left: 3px solid var(--err); border-radius: 3px; color: var(--err); font-size: 0.8rem; }
</style>

<script lang="ts">
	// GlobalSearch : modal de recherche invoquée par `/` ou `Cmd+K`.
	// Cherche en live sur projects + epics + stories via
	// /api/v1/search. Résultats navigables au clavier (↑↓ + Enter).
	//
	// Ouverte par keyboard shortcut (géré dans +layout.svelte) ;
	// close par Esc ou click backdrop.

	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { apiGet } from './api';

	type Hit = {
		type: 'project' | 'epic' | 'story';
		id: string;
		title: string;
		subtitle?: string;
		project_id?: string;
	};

	type Props = { open: boolean; onClose: () => void };
	let { open = $bindable(), onClose }: Props = $props();

	let q = $state('');
	let hits = $state<Hit[]>([]);
	let cursor = $state(0);
	let inputEl: HTMLInputElement | undefined = $state(undefined);
	let debounce: ReturnType<typeof setTimeout> | null = null;

	$effect(() => {
		if (open) {
			queueMicrotask(() => inputEl?.focus());
		} else {
			q = '';
			hits = [];
			cursor = 0;
		}
	});

	// Debounce la recherche pour ne pas marteler l'API à chaque frappe.
	$effect(() => {
		if (!open) return;
		const term = q.trim();
		if (term.length < 2) {
			hits = [];
			return;
		}
		if (debounce) clearTimeout(debounce);
		debounce = setTimeout(async () => {
			try {
				hits = (await apiGet<Hit[]>(`/api/v1/search?q=${encodeURIComponent(term)}`)) ?? [];
				cursor = 0;
			} catch {
				hits = [];
			}
		}, 120);
	});

	function navigateTo(h: Hit) {
		if (h.type === 'project') {
			goto(`/projects/${encodeURIComponent(h.id)}`);
		} else if (h.project_id) {
			goto(`/projects/${encodeURIComponent(h.project_id)}`);
		}
		onClose();
	}

	function onKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.preventDefault();
			onClose();
		} else if (e.key === 'ArrowDown') {
			e.preventDefault();
			cursor = Math.min(hits.length - 1, cursor + 1);
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			cursor = Math.max(0, cursor - 1);
		} else if (e.key === 'Enter' && hits[cursor]) {
			e.preventDefault();
			navigateTo(hits[cursor]);
		}
	}

	onMount(() => {
		window.addEventListener('keydown', onKeydown);
		return () => window.removeEventListener('keydown', onKeydown);
	});

	function iconFor(type: Hit['type']): string {
		return type === 'project' ? '▦' : type === 'epic' ? '◈' : '○';
	}
</script>

{#if open}
	<div class="backdrop" role="presentation" onclick={onClose}></div>
	<aside class="search-panel" aria-label="Recherche globale">
		<div class="input-row">
			<span class="prefix">⌕</span>
			<input
				bind:this={inputEl}
				bind:value={q}
				type="search"
				placeholder="Chercher un projet, epic, story…"
				aria-label="Terme de recherche"
			/>
			<kbd class="esc-hint">Esc</kbd>
		</div>
		{#if q.trim().length >= 2 && hits.length === 0}
			<div class="empty">Aucun résultat pour « {q} »</div>
		{:else if hits.length > 0}
			<ul class="hits" role="listbox">
				{#each hits as h, i (h.type + ':' + h.id)}
					<li role="option" aria-selected={i === cursor}>
						<button type="button"
							class:active={i === cursor}
							onclick={() => navigateTo(h)}
							onmouseenter={() => (cursor = i)}
							onfocus={() => (cursor = i)}>
							<span class="icon" aria-hidden="true">{iconFor(h.type)}</span>
							<span class="type">{h.type}</span>
							<span class="title">{h.title}</span>
							{#if h.subtitle}
								<span class="sub">{h.subtitle}</span>
							{/if}
						</button>
					</li>
				{/each}
			</ul>
		{:else}
			<p class="hint">
				Tape au moins 2 caractères. <kbd>↑</kbd> <kbd>↓</kbd> pour naviguer, <kbd>Entrée</kbd> pour ouvrir.
			</p>
		{/if}
	</aside>
{/if}

<style>
	.backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.45);
		z-index: 300;
	}
	.search-panel {
		position: fixed;
		top: 8vh;
		left: 50%;
		transform: translateX(-50%);
		width: min(600px, 92vw);
		max-height: 70vh;
		display: flex;
		flex-direction: column;
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 10px;
		box-shadow: 0 24px 60px rgba(0, 0, 0, 0.35);
		z-index: 301;
	}
	.input-row {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		padding: 0.75rem 1rem;
		border-bottom: 1px solid var(--border);
	}
	.input-row .prefix {
		font-size: 1.1rem;
		color: var(--text-muted);
	}
	.input-row input {
		flex: 1;
		border: none;
		background: transparent;
		color: var(--text);
		font-size: 1rem;
		font-family: inherit;
		outline: none;
	}
	.esc-hint {
		font-size: 0.7rem;
		padding: 2px 6px;
		background: var(--bg-alt, var(--bg));
		color: var(--text-muted);
		border: 1px solid var(--border);
		border-radius: 4px;
	}
	.hits {
		list-style: none;
		padding: 0.3rem;
		margin: 0;
		overflow-y: auto;
		flex: 1;
	}
	.hits li {
		list-style: none;
	}
	.hits button {
		display: grid;
		grid-template-columns: 24px 72px 1fr auto;
		align-items: center;
		gap: 0.6rem;
		padding: 0.5rem 0.65rem;
		border-radius: 6px;
		cursor: pointer;
		font-size: 0.87rem;
		width: 100%;
		background: transparent;
		border: none;
		color: inherit;
		font-family: inherit;
		text-align: left;
	}
	.hits button.active {
		background: color-mix(in srgb, var(--accent) 14%, transparent);
	}
	.hits .icon {
		color: var(--text-muted);
		text-align: center;
	}
	.hits .type {
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--text-muted);
	}
	.hits .title {
		color: var(--text);
		font-weight: 500;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.hits .sub {
		color: var(--text-muted);
		font-size: 0.75rem;
		text-align: right;
		max-width: 180px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.empty, .hint {
		padding: 1rem 1.25rem;
		color: var(--text-muted);
		font-size: 0.85rem;
	}
	.hint kbd {
		font-size: 0.72rem;
		padding: 1px 5px;
		background: var(--bg-alt, var(--bg));
		border: 1px solid var(--border);
		border-radius: 3px;
		margin: 0 2px;
	}
</style>

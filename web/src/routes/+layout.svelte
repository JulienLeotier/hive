<script lang="ts">
	import { onMount } from 'svelte';
	import favicon from '$lib/assets/favicon.svg';
	import { page } from '$app/stores';
	import { afterNavigate } from '$app/navigation';
	import { theme, toggleTheme, applyStoredTheme } from '$lib/theme';
	import { apiError } from '$lib/api';
	import { wsStatus } from '$lib/wsStatus';

	let { children } = $props();

	// Scroll-to-top on chaque nav client-side. SvelteKit le fait par
	// défaut quand l'URL change, mais pas sur un simple changement de
	// query-string — on garantit le comportement explicitement pour
	// éviter les surprises quand on revient d'une page longue sur une
	// autre (ex. /projects/[id] → /projects).
	afterNavigate(({ from, to }) => {
		if (from && to && from.url.pathname !== to.url.pathname) {
			window.scrollTo({ top: 0, behavior: 'instant' });
		}
	});

	function wsStatusLabel(s: 'connecting' | 'open' | 'closed'): string {
		return s === 'open' ? 'live' : s === 'connecting' ? 'connexion…' : 'hors-ligne';
	}

	// BMAD-mode nav. The product is a local, single-user product factory:
	// one idea in, one shipped product out. Everything in Build drives a
	// project; Inspect is the debug/observability catch-all for when
	// something goes sideways. The "Fleet" group (agents/playground/
	// knowledge) belonged to the pre-pivot multi-agent platform and is
	// gone from nav — the route files stay for now to avoid touching
	// unrelated code but they are no longer part of the product surface.
	const navGroups = [
		{
			label: 'Construction',
			items: [
				{ href: '/', label: 'Accueil' },
				{ href: '/projects', label: 'Projets' }
			]
		},
		{
			label: 'Inspection',
			items: [
				{ href: '/events', label: 'Événements' },
				{ href: '/audit', label: 'Audit' },
				{ href: '/costs', label: 'Coûts' }
			]
		},
		{
			label: 'Système',
			items: [
				{ href: '/settings', label: 'Réglages' }
			]
		}
	];

	onMount(() => {
		applyStoredTheme();
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<div class="app">
	<aside class="sidebar">
		<a href="/" class="brand">
			<span class="logo">⬡</span>
			<span class="brand-text">Hive</span>
		</a>
		<nav>
			{#each navGroups as group}
				<div class="group">
					<span class="group-label">{group.label}</span>
					{#each group.items as item}
						<a href={item.href} class:active={$page.url.pathname === item.href}>
							{item.label}
						</a>
					{/each}
				</div>
			{/each}
		</nav>
		<div class="sidebar-footer">
			<div class="ws-status" title={wsStatusLabel($wsStatus)}>
				<span class="ws-dot" class:open={$wsStatus === 'open'}
					class:connecting={$wsStatus === 'connecting'}
					class:closed={$wsStatus === 'closed'}></span>
				<span class="ws-label">{wsStatusLabel($wsStatus)}</span>
			</div>
			<button class="theme-toggle" onclick={toggleTheme} title="Toggle dark mode">
				{$theme === 'dark' ? '☀' : '☾'}
			</button>
		</div>
	</aside>
	<main class="content">
		{#if $apiError}
			<div class="api-banner" role="alert">
				<span class="dot"></span>
				<span class="msg">Serveur injoignable — {$apiError}</span>
				<button class="dismiss" onclick={() => apiError.set(null)} aria-label="Dismiss">×</button>
			</div>
		{/if}
		{@render children()}
	</main>
</div>

<style>
	:global(:root) {
		--bg: #fafafa;
		--bg-panel: #ffffff;
		--bg-hover: #f1f5f9;
		--text: #0f172a;
		--text-muted: #64748b;
		--border: #e5e7eb;
		--accent: #3b82f6;
		--accent-dim: #60a5fa;
		--ok: #22c55e;
		--warn: #f59e0b;
		--err: #ef4444;
	}
	:global([data-theme='dark']) {
		--bg: #0b1220;
		--bg-panel: #111827;
		--bg-hover: #1f2937;
		--text: #f1f5f9;
		--text-muted: #94a3b8;
		--border: #1f2937;
		--accent: #60a5fa;
		--accent-dim: #3b82f6;
	}

	:global(html),
	:global(body) {
		margin: 0;
		font-family: system-ui, -apple-system, sans-serif;
		background: var(--bg);
		color: var(--text);
		transition: background 0.15s, color 0.15s;
	}

	:global(table) {
		width: 100%;
		border-collapse: collapse;
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 8px;
		overflow: hidden;
	}
	:global(th),
	:global(td) {
		padding: 0.5rem 0.75rem;
		text-align: left;
		border-bottom: 1px solid var(--border);
	}
	:global(th) {
		font-weight: 600;
		color: var(--text-muted);
		background: var(--bg);
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}
	:global(tr:last-child td) {
		border-bottom: none;
	}
	:global(code) {
		font-size: 0.75rem;
		background: var(--bg-hover);
		padding: 1px 6px;
		border-radius: 3px;
		color: var(--accent);
	}
	:global(h1) {
		margin: 0 0 0.25rem;
		font-size: 1.5rem;
	}
	:global(h2) {
		margin: 1.5rem 0 0.5rem;
		font-size: 1rem;
		color: var(--text-muted);
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}
	:global(.empty) {
		color: var(--text-muted);
		font-style: italic;
		padding: 1rem;
		border: 1px dashed var(--border);
		border-radius: 8px;
		text-align: center;
	}
	:global(.badge) {
		display: inline-block;
		padding: 2px 8px;
		border-radius: 4px;
		color: white;
		font-size: 0.7rem;
		font-weight: 500;
	}

	.app {
		display: grid;
		grid-template-columns: 220px 1fr;
		min-height: 100vh;
	}
	.sidebar {
		background: var(--bg-panel);
		border-right: 1px solid var(--border);
		padding: 1rem;
		display: flex;
		flex-direction: column;
		gap: 1.5rem;
	}
	.brand {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		text-decoration: none;
		color: var(--text);
		font-weight: 700;
		font-size: 1.25rem;
	}
	.logo {
		color: var(--accent);
		font-size: 1.5rem;
	}
	nav {
		display: flex;
		flex-direction: column;
		gap: 1.25rem;
		flex: 1;
	}
	.group {
		display: flex;
		flex-direction: column;
		gap: 0.125rem;
	}
	.group-label {
		font-size: 0.7rem;
		color: var(--text-muted);
		text-transform: uppercase;
		letter-spacing: 0.08em;
		padding: 0 0.5rem;
		margin-bottom: 0.25rem;
	}
	.group a {
		display: block;
		color: var(--text-muted);
		text-decoration: none;
		padding: 0.375rem 0.75rem;
		border-radius: 4px;
		font-size: 0.875rem;
		transition: background 0.1s, color 0.1s;
	}
	.group a:hover {
		background: var(--bg-hover);
		color: var(--text);
	}
	.group a.active {
		background: var(--accent);
		color: white;
	}
	.sidebar-footer {
		display: flex;
		gap: 0.5rem;
		align-items: center;
	}
	.ws-status {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		font-size: 0.7rem;
		color: var(--text-muted);
		flex: 1;
	}
	.ws-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background: var(--text-muted);
		flex-shrink: 0;
	}
	.ws-dot.open { background: var(--ok); box-shadow: 0 0 4px var(--ok); }
	.ws-dot.connecting { background: var(--warn); animation: pulse 1s ease-in-out infinite; }
	.ws-dot.closed { background: var(--err); }
	@keyframes pulse {
		0%, 100% { opacity: 1; }
		50% { opacity: 0.3; }
	}
	.ws-label { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
	.theme-toggle {
		background: transparent;
		border: 1px solid var(--border);
		color: var(--text);
		padding: 0.5rem;
		border-radius: 6px;
		cursor: pointer;
		font-size: 1rem;
	}
	.theme-toggle:hover {
		background: var(--bg-hover);
	}
	.content {
		padding: 2rem 2.5rem;
		max-width: 1400px;
	}
	.api-banner {
		display: flex;
		align-items: center;
		gap: 0.625rem;
		background: color-mix(in srgb, var(--err) 12%, var(--bg-panel));
		border: 1px solid var(--err);
		color: var(--text);
		padding: 0.5rem 0.75rem;
		border-radius: 6px;
		font-size: 0.8rem;
		margin-bottom: 1rem;
	}
	.api-banner .dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background: var(--err);
		flex-shrink: 0;
	}
	.api-banner .msg {
		flex: 1;
		font-family: ui-monospace, monospace;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.api-banner .dismiss {
		background: transparent;
		border: none;
		color: var(--text-muted);
		font-size: 1.1rem;
		line-height: 1;
		cursor: pointer;
		padding: 0 0.25rem;
	}
	.api-banner .dismiss:hover {
		color: var(--text);
	}
</style>

<script lang="ts">
	import { onMount } from 'svelte';
	import favicon from '$lib/assets/favicon.svg';
	import { page } from '$app/stores';
	import { afterNavigate } from '$app/navigation';
	import { theme, toggleTheme, applyStoredTheme } from '$lib/theme';
	import { apiError } from '$lib/api';
	import { wsStatus } from '$lib/wsStatus';

	let { children } = $props();

	// État drawer mobile : fermé par défaut, ouvert quand on clique le
	// bouton hamburger. Sur desktop la sidebar est toujours visible
	// donc drawerOpen est ignoré par les media queries.
	let drawerOpen = $state(false);

	// Scroll-to-top + close drawer sur chaque nav client-side.
	afterNavigate(({ from, to }) => {
		if (from?.url && to?.url && from.url.pathname !== to.url.pathname) {
			window.scrollTo({ top: 0, behavior: 'instant' });
			drawerOpen = false;
		}
	});

	function wsStatusLabel(s: 'connecting' | 'open' | 'closed'): string {
		return s === 'open' ? 'live' : s === 'connecting' ? 'connexion…' : 'hors-ligne';
	}

	const navGroups = [
		{
			label: 'Construction',
			items: [
				{ href: '/', label: 'Accueil', icon: '⌂' },
				{ href: '/projects', label: 'Projets', icon: '▤' }
			]
		},
		{
			label: 'Inspection',
			items: [
				{ href: '/events', label: 'Événements', icon: '◈' },
				{ href: '/audit', label: 'Audit', icon: '✓' },
				{ href: '/costs', label: 'Coûts', icon: '$' }
			]
		},
		{
			label: 'Système',
			items: [
				{ href: '/settings', label: 'Réglages', icon: '⚙' }
			]
		}
	];

	// Titre courant pour le header mobile (pas de sidebar visible).
	let currentPageLabel = $derived.by(() => {
		const path = $page.url.pathname;
		for (const g of navGroups) {
			for (const i of g.items) {
				if (i.href === path) return i.label;
			}
		}
		if (path.startsWith('/projects/')) return 'Projet';
		return 'Hive';
	});

	onMount(() => {
		applyStoredTheme();
		// Fermer le drawer avec Escape.
		const onKey = (e: KeyboardEvent) => {
			if (e.key === 'Escape') drawerOpen = false;
		};
		window.addEventListener('keydown', onKey);
		return () => window.removeEventListener('keydown', onKey);
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover" />
	<meta name="theme-color" content="#111827" />
</svelte:head>

<!-- Header mobile uniquement (caché en ≥768px via CSS). -->
<header class="topbar">
	<button
		class="burger"
		onclick={() => (drawerOpen = !drawerOpen)}
		aria-label="Menu"
		aria-expanded={drawerOpen}>
		<span></span><span></span><span></span>
	</button>
	<a href="/" class="topbar-brand">
		<span class="logo">⬡</span>
		<span>Hive</span>
	</a>
	<span class="topbar-title">{currentPageLabel}</span>
	<button class="theme-toggle sm" onclick={toggleTheme} aria-label="Theme">
		{$theme === 'dark' ? '☀' : '☾'}
	</button>
</header>

<div class="app" class:drawer-open={drawerOpen}>
	<!-- Overlay mobile pour fermer le drawer au tap. -->
	{#if drawerOpen}
		<button class="drawer-overlay"
			onclick={() => (drawerOpen = false)}
			aria-label="Fermer le menu"></button>
	{/if}

	<aside class="sidebar" class:open={drawerOpen}>
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
							<span class="nav-icon" aria-hidden="true">{item.icon}</span>
							<span class="nav-text">{item.label}</span>
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

		/* Touch targets + safe-area iOS. */
		--tap-min: 44px;
		--safe-top: env(safe-area-inset-top, 0);
		--safe-bottom: env(safe-area-inset-bottom, 0);
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
		-webkit-text-size-adjust: 100%;
		-webkit-tap-highlight-color: transparent;
	}
	:global(body) { overscroll-behavior-y: none; }

	/* Tables — sur desktop, rendu classique. Sur mobile, .table-responsive
	   dans les pages convertit en cards ; en fallback on ajoute
	   scroll-x horizontal. */
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
		word-break: break-all;
	}
	:global(h1) {
		margin: 0 0 0.25rem;
		font-size: 1.5rem;
		line-height: 1.2;
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
	/* Utility : scroll-x pour les tables qui n'ont pas été converties en cards. */
	:global(.table-scroll) {
		overflow-x: auto;
		-webkit-overflow-scrolling: touch;
		border-radius: 8px;
	}
	/* Touch-friendly : tous les boutons ≥44px de haut en tap. */
	:global(button:not(.link):not(.crumb):not(.close):not(.dismiss)),
	:global(a.btn) {
		min-height: var(--tap-min);
	}

	/* ========== Topbar (mobile only) ========== */
	.topbar {
		display: none;
		position: sticky;
		top: 0;
		z-index: 100;
		padding: calc(var(--safe-top) + 0.4rem) 0.5rem 0.4rem;
		background: var(--bg-panel);
		border-bottom: 1px solid var(--border);
		align-items: center;
		gap: 0.5rem;
	}
	.burger {
		background: transparent;
		border: none;
		color: var(--text);
		padding: 0.5rem;
		width: 44px;
		height: 44px;
		display: flex;
		flex-direction: column;
		justify-content: center;
		gap: 4px;
		cursor: pointer;
		border-radius: 6px;
	}
	.burger span {
		display: block;
		width: 22px;
		height: 2px;
		background: var(--text);
		border-radius: 2px;
	}
	.burger:hover { background: var(--bg-hover); }
	.topbar-brand {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		text-decoration: none;
		color: var(--text);
		font-weight: 700;
	}
	.topbar-title {
		flex: 1;
		text-align: center;
		font-size: 0.9rem;
		color: var(--text-muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.theme-toggle.sm {
		width: 44px;
		height: 44px;
		padding: 0;
		display: flex;
		align-items: center;
		justify-content: center;
	}

	/* ========== Layout grid ========== */
	.app {
		display: grid;
		grid-template-columns: 220px 1fr;
		min-height: 100vh;
	}
	.drawer-overlay {
		display: none; /* visible only on mobile via media query below */
	}

	/* ========== Sidebar ========== */
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
		display: flex;
		align-items: center;
		gap: 0.65rem;
		color: var(--text-muted);
		text-decoration: none;
		padding: 0.55rem 0.75rem;
		border-radius: 4px;
		font-size: 0.875rem;
		transition: background 0.1s, color 0.1s;
		min-height: var(--tap-min);
	}
	.nav-icon {
		width: 18px;
		text-align: center;
		font-size: 0.95rem;
		opacity: 0.9;
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

	/* ========== Content ========== */
	.content {
		padding: 2rem 2.5rem;
		max-width: 1400px;
		min-width: 0; /* autoriser text-overflow + table-scroll */
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
		font-size: 1.25rem;
		line-height: 1;
		cursor: pointer;
		padding: 0 0.5rem;
		width: 32px;
		height: 32px;
	}
	.api-banner .dismiss:hover {
		color: var(--text);
	}

	/* ========== Mobile ≤ 767px : drawer comportement ========== */
	@media (max-width: 767px) {
		.topbar { display: flex; }
		.app {
			grid-template-columns: 1fr;
		}
		.sidebar {
			position: fixed;
			top: 0;
			left: 0;
			bottom: 0;
			width: min(280px, 80vw);
			padding: calc(var(--safe-top) + 1rem) 1rem calc(var(--safe-bottom) + 1rem);
			transform: translateX(-100%);
			transition: transform 0.22s ease;
			z-index: 200;
			overflow-y: auto;
			box-shadow: 2px 0 20px rgba(0, 0, 0, 0.12);
		}
		.sidebar.open { transform: translateX(0); }
		.drawer-overlay {
			display: block;
			position: fixed;
			inset: 0;
			background: rgba(0, 0, 0, 0.45);
			z-index: 150;
			border: none;
			padding: 0;
			cursor: pointer;
			animation: fade-in 0.15s ease;
		}
		@keyframes fade-in {
			from { opacity: 0; }
			to { opacity: 1; }
		}
		.content {
			padding: 1rem 1rem calc(1rem + var(--safe-bottom));
		}
		.group a { font-size: 0.95rem; }

		/* Les H1/H2 gagnent un peu de punch sur mobile. */
		:global(h1) { font-size: 1.35rem; }
		:global(h2) { margin-top: 1.25rem; }

		/* Tables scroll-x de base sur mobile, au cas où. */
		:global(table) { font-size: 0.82rem; }
		:global(th), :global(td) { padding: 0.5rem 0.6rem; }
	}

	/* Desktop ≥ 768px : sidebar toujours visible (reset transform). */
	@media (min-width: 768px) {
		.sidebar { transform: none !important; }
	}

	/* Tablette ≥ 768px mais < 1024px : légère réduction du padding. */
	@media (min-width: 768px) and (max-width: 1023px) {
		.content { padding: 1.5rem 1.75rem; }
	}
</style>

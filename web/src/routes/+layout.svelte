<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import favicon from '$lib/assets/favicon.svg';
	import { page } from '$app/stores';
	import { theme, toggleTheme, applyStoredTheme } from '$lib/theme';
	import { apiError, getStoredKey, clearStoredKey } from '$lib/api';

	let { children } = $props();

	// On a fresh deployment, redirect to /setup before showing any page so
	// the user can't land on an empty dashboard and wonder why nothing
	// loads. Once setup is done and no key is stored, send the user to
	// /login instead — otherwise every page would immediately 401-loop.
	async function gateAccess() {
		const path = $page.url.pathname;
		try {
			const r = await fetch('/api/v1/setup/status');
			if (!r.ok) return;
			const json = await r.json();
			const needsSetup = json?.data?.needs_setup === true;

			if (needsSetup && path !== '/setup') {
				goto('/setup');
				return;
			}
			if (!needsSetup && !getStoredKey() && path !== '/login' && path !== '/setup') {
				goto('/login');
			}
		} catch {
			/* network down — the global banner will flag it */
		}
	}

	function signOut() {
		clearStoredKey();
		goto('/login');
	}

	// BMAD-mode nav: only what an operator driving an autonomous product
	// build needs. Enterprise features (federation, marketplace, billing,
	// multi-tenant, webhooks, cluster) are kept in the code but hidden
	// because this tool is local, single-user, single-project-at-a-time.
	// Flip HIVE_ENTERPRISE=1 on the build to expose them — see the Info
	// doc for rationale.
	const ENTERPRISE_MODE =
		typeof window !== 'undefined' && localStorage.getItem('hive.mode') === 'enterprise';

	const bmadNav = [
		{
			label: 'Build',
			items: [
				{ href: '/', label: 'Home' },
				{ href: '/projects', label: 'Projects' }
			]
		},
		{
			label: 'Fleet',
			items: [
				{ href: '/agents', label: 'Agents' },
				{ href: '/playground', label: 'Playground' },
				{ href: '/knowledge', label: 'Knowledge' }
			]
		},
		{
			label: 'Inspect',
			items: [
				{ href: '/tasks', label: 'Tasks' },
				{ href: '/events', label: 'Events' },
				{ href: '/costs', label: 'Costs' },
				{ href: '/audit', label: 'Audit' }
			]
		}
	];

	const enterpriseNav = [
		{
			label: 'Orchestration',
			items: [
				{ href: '/workflows', label: 'Workflows' }
			]
		},
		{
			label: 'Economy',
			items: [
				{ href: '/billing', label: 'Billing' },
				{ href: '/market', label: 'Market' },
				{ href: '/optimizer', label: 'Optimizer' }
			]
		},
		{
			label: 'Operations',
			items: [
				{ href: '/cluster', label: 'Cluster' },
				{ href: '/federation', label: 'Federation' },
				{ href: '/marketplace', label: 'Marketplace' },
				{ href: '/webhooks', label: 'Webhooks' },
				{ href: '/dialogs', label: 'Dialogs' }
			]
		},
		{
			label: 'Governance',
			items: [
				{ href: '/users', label: 'Users' },
				{ href: '/tenants', label: 'Tenants' },
				{ href: '/trust', label: 'Trust' }
			]
		}
	];

	const navGroups = ENTERPRISE_MODE ? [...bmadNav, ...enterpriseNav] : bmadNav;

	onMount(() => {
		applyStoredTheme();
		gateAccess();
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

{#if $page.url.pathname === '/setup' || $page.url.pathname === '/login'}
	<!-- Pre-auth wizard / login: no sidebar, no nav — just the form so
	     the user isn't distracted by 16 empty pages. -->
	{@render children()}
{:else}
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
			<button class="theme-toggle" onclick={toggleTheme} title="Toggle dark mode">
				{$theme === 'dark' ? '☀' : '☾'}
			</button>
			<button class="sign-out" onclick={signOut} title="Sign out">
				⎋
			</button>
		</div>
	</aside>
	<main class="content">
		{#if $apiError}
			<div class="api-banner" role="alert">
				<span class="dot"></span>
				<span class="msg">Backend unreachable — {$apiError}</span>
				<button class="dismiss" onclick={() => apiError.set(null)} aria-label="Dismiss">×</button>
			</div>
		{/if}
		{@render children()}
	</main>
</div>
{/if}

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
	}
	.theme-toggle,
	.sign-out {
		background: transparent;
		border: 1px solid var(--border);
		color: var(--text);
		padding: 0.5rem;
		border-radius: 6px;
		cursor: pointer;
		font-size: 1rem;
	}
	.theme-toggle:hover,
	.sign-out:hover {
		background: var(--bg-hover);
	}
	.sign-out:hover {
		color: var(--err);
		border-color: var(--err);
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

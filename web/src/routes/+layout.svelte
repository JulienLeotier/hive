<script lang="ts">
	import favicon from '$lib/assets/favicon.svg';
	import { page } from '$app/stores';

	let { children } = $props();

	const navItems = [
		{ href: '/', label: 'Dashboard' },
		{ href: '/agents', label: 'Agents' },
		{ href: '/tasks', label: 'Tasks' },
		{ href: '/events', label: 'Events' },
		{ href: '/costs', label: 'Costs' },
	];
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<div class="app">
	<nav class="topnav">
		<a href="/" class="brand">Hive</a>
		<div class="nav-links">
			{#each navItems as item}
				<a href={item.href} class:active={$page.url.pathname === item.href}>{item.label}</a>
			{/each}
		</div>
	</nav>
	<div class="content">
		{@render children()}
	</div>
</div>

<style>
	:global(body) { margin: 0; font-family: system-ui, -apple-system, sans-serif; }
	.app { min-height: 100vh; }
	.topnav {
		display: flex;
		align-items: center;
		gap: 2rem;
		padding: 0.75rem 2rem;
		background: #1a1a1a;
		color: white;
	}
	.brand {
		font-weight: 700;
		font-size: 1.25rem;
		color: white;
		text-decoration: none;
	}
	.nav-links { display: flex; gap: 1rem; }
	.nav-links a {
		color: #aaa;
		text-decoration: none;
		padding: 0.25rem 0.5rem;
		border-radius: 4px;
		font-size: 0.875rem;
	}
	.nav-links a:hover { color: white; }
	.nav-links a.active { color: white; background: #333; }
	.content { max-width: 1200px; margin: 0 auto; padding: 2rem; }
</style>

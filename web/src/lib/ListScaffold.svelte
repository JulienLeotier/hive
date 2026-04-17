<!--
  Shared wrapper for dashboard list pages. Each page supplies its own title,
  subtitle, and body (snippet); the scaffold handles the loading spinner and
  empty state messaging that were otherwise copy-pasted across 13 pages.

  Usage:

    <ListScaffold title="Cluster" subtitle="..." {loading} isEmpty={members.length === 0} emptyText="...">
      {#snippet controls()}<div class="filter">...</div>{/snippet}
      <table>...</table>
    </ListScaffold>
-->
<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		title,
		subtitle = '',
		loading = false,
		isEmpty = false,
		emptyText = 'No entries.',
		controls,
		children
	}: {
		title: string;
		subtitle?: string;
		loading?: boolean;
		isEmpty?: boolean;
		emptyText?: string;
		controls?: Snippet;
		children: Snippet;
	} = $props();
</script>

<main>
	<h1>{title}</h1>
	{#if subtitle}<p class="subtitle">{subtitle}</p>{/if}
	{#if controls}{@render controls()}{/if}

	{#if loading}
		<div class="empty">Loading…</div>
	{:else if isEmpty}
		<div class="empty">{emptyText}</div>
	{:else}
		{@render children()}
	{/if}
</main>

<style>
	.subtitle {
		color: var(--text-muted);
		margin-top: 0;
	}
</style>

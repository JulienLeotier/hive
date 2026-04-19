<script lang="ts">
	import { onMount } from 'svelte';

	// Render l'OpenAPI local sans dépendance externe (pas de CDN
	// Swagger qui vient contredire le "single binary"). On fetch le
	// yaml brut et on le rend avec une liste compacte des endpoints
	// + le full YAML déroulable.

	type Endpoint = { method: string; path: string; summary?: string };

	let endpoints = $state<Endpoint[]>([]);
	let rawYaml = $state('');
	let expanded = $state(false);

	onMount(async () => {
		try {
			const res = await fetch('/api/openapi.yaml');
			const text = await res.text();
			rawYaml = text;
			endpoints = parseEndpoints(text);
		} catch {
			rawYaml = '# Impossible de charger /api/openapi.yaml';
		}
	});

	// Parser minimaliste : walk les lignes, détecte les paths (indent 2,
	// commence par /) et les méthodes (indent 4 : get/post/...).
	function parseEndpoints(yaml: string): Endpoint[] {
		const out: Endpoint[] = [];
		const methods = ['get', 'post', 'put', 'patch', 'delete'];
		const lines = yaml.split('\n');
		let inPaths = false;
		let currentPath = '';
		let lastMethod: Endpoint | null = null;
		for (const raw of lines) {
			if (/^paths:\s*$/.test(raw)) {
				inPaths = true;
				continue;
			}
			if (!inPaths) continue;
			// Nouveau groupe de top-level (ex "components:") → stop.
			if (/^[a-z]/.test(raw) && !/^(components|tags|info|servers):$/.test(raw)) {
				// keep going — paths cohabite avec d'autres sections
			}
			// Path : "  /api/v1/projects:" (2 espaces, finit par :)
			const p = raw.match(/^  (\/[^:]+):\s*$/);
			if (p) {
				currentPath = p[1];
				lastMethod = null;
				continue;
			}
			// Méthode : "    get:" (4 espaces)
			const m = raw.match(/^    (get|post|put|patch|delete):\s*$/);
			if (m && methods.includes(m[1]) && currentPath) {
				lastMethod = { method: m[1].toUpperCase(), path: currentPath };
				out.push(lastMethod);
				continue;
			}
			// Summary : "      summary: …"
			const s = raw.match(/^      summary:\s*(.*)$/);
			if (s && lastMethod) {
				lastMethod.summary = s[1].replace(/^["']|["']$/g, '');
			}
		}
		return out;
	}

	function methodColor(m: string): string {
		switch (m) {
			case 'GET': return 'var(--accent)';
			case 'POST': return '#16a34a';
			case 'PATCH': return '#f59e0b';
			case 'PUT': return '#f59e0b';
			case 'DELETE': return 'var(--err, #dc2626)';
			default: return 'var(--text-muted)';
		}
	}
</script>

<svelte:head><title>API · Hive</title></svelte:head>

<h1>API Hive</h1>
<p class="sub">
	{endpoints.length} endpoint{endpoints.length > 1 ? 's' : ''} exposés par <code>hive serve</code>.
	Fichier brut : <a href="/api/openapi.yaml" download>openapi.yaml</a>.
</p>

<section class="endpoints">
	{#each endpoints as e (e.method + e.path)}
		<div class="ep">
			<span class="method" style="background:{methodColor(e.method)}">{e.method}</span>
			<code class="path">{e.path}</code>
			{#if e.summary}<span class="sum">— {e.summary}</span>{/if}
		</div>
	{:else}
		<div class="empty">Chargement…</div>
	{/each}
</section>

<details class="raw" bind:open={expanded}>
	<summary>Voir le YAML complet</summary>
	<pre>{rawYaml}</pre>
</details>

<style>
	.sub {
		color: var(--text-muted);
		font-size: 0.9rem;
		margin-bottom: 1.25rem;
	}
	.endpoints {
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
	}
	.ep {
		display: grid;
		grid-template-columns: 68px auto 1fr;
		align-items: baseline;
		gap: 0.7rem;
		padding: 0.5rem 0.75rem;
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 6px;
		font-size: 0.85rem;
	}
	.method {
		text-align: center;
		padding: 2px 6px;
		color: #fff;
		border-radius: 4px;
		font-weight: 700;
		font-size: 0.72rem;
		letter-spacing: 0.03em;
	}
	.path {
		font-family: ui-monospace, monospace;
		color: var(--text);
	}
	.sum {
		color: var(--text-muted);
		font-size: 0.82rem;
	}
	.empty {
		padding: 1.5rem;
		color: var(--text-muted);
		text-align: center;
	}
	.raw {
		margin-top: 1.5rem;
	}
	.raw summary {
		cursor: pointer;
		color: var(--text-muted);
		font-size: 0.85rem;
		padding: 0.5rem 0;
	}
	.raw pre {
		font-family: ui-monospace, monospace;
		font-size: 0.78rem;
		line-height: 1.55;
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 1rem;
		overflow-x: auto;
		white-space: pre;
		max-height: 600px;
	}
</style>

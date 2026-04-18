<script lang="ts">
	import { page } from '$app/stores';
	import { apiGet } from '$lib/api';
	import { createReconnectingWS, wsURL } from '$lib/ws';

	type FileEntry = {
		path: string;
		size: number;
		is_dir: boolean;
		modified: string;
	};
	type ListResponse = {
		root: string;
		files: FileEntry[];
		truncated?: boolean;
		message?: string;
	};
	type ContentResponse = {
		path: string;
		size: number;
		is_binary: boolean;
		truncated?: boolean;
		content?: string;
	};

	let listing = $state<ListResponse | null>(null);
	let listingError = $state('');
	let selected = $state<string>('');
	let content = $state<ContentResponse | null>(null);
	let contentError = $state('');
	let contentLoading = $state(false);
	let search = $state('');

	async function loadList() {
		const id = $page.params.id ?? '';
		if (!id) return;
		try {
			listing = await apiGet<ListResponse>(`/api/v1/projects/${encodeURIComponent(id)}/files`);
		} catch (e) {
			listingError = e instanceof Error ? e.message : String(e);
		}
	}

	async function loadContent(path: string) {
		const id = $page.params.id ?? '';
		if (!id || !path) return;
		contentError = '';
		contentLoading = true;
		try {
			const qs = new URLSearchParams({ path }).toString();
			content = await apiGet<ContentResponse>(
				`/api/v1/projects/${encodeURIComponent(id)}/files/content?${qs}`
			);
		} catch (e) {
			contentError = e instanceof Error ? e.message : String(e);
			content = null;
		} finally {
			contentLoading = false;
		}
	}

	function pickFile(path: string) {
		selected = path;
		loadContent(path);
	}

	function clearSelection() {
		selected = '';
		content = null;
	}

	function copyContent() {
		if (content?.content) {
			navigator.clipboard.writeText(content.content);
		}
	}

	$effect(() => {
		loadList();
		const ws = createReconnectingWS({
			url: wsURL('/ws'),
			onmessage: (msg) => {
				try {
					const evt = JSON.parse(msg.data) as { type?: string };
					if (!evt.type) return;
					if (evt.type.startsWith('story.') || evt.type === 'project.shipped') {
						loadList();
					}
				} catch {
					/* ignore */
				}
			}
		});
		return () => ws.close();
	});

	// Group files by top-level directory for a compact tree.
	let grouped = $derived.by(() => {
		const needle = search.trim().toLowerCase();
		const filtered = needle
			? (listing?.files ?? []).filter((f) => f.path.toLowerCase().includes(needle))
			: (listing?.files ?? []);
		const groups = new Map<string, FileEntry[]>();
		for (const f of filtered) {
			const i = f.path.indexOf('/');
			const top = i < 0 ? '' : f.path.slice(0, i);
			if (!groups.has(top)) groups.set(top, []);
			groups.get(top)!.push(f);
		}
		return [...groups.entries()].sort(([a], [b]) => a.localeCompare(b));
	});

	let fileCount = $derived(listing?.files?.length ?? 0);
	let filteredCount = $derived(grouped.reduce((n, [, files]) => n + files.length, 0));

	function fmtSize(n: number): string {
		if (n < 1024) return `${n} B`;
		if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
		return `${(n / (1024 * 1024)).toFixed(1)} MB`;
	}

	function fileIcon(path: string, isDir: boolean): string {
		if (isDir) return '📁';
		const ext = path.split('.').pop()?.toLowerCase() ?? '';
		const map: Record<string, string> = {
			go: '🔵', ts: '🟦', tsx: '🟦', js: '🟨', jsx: '🟨',
			svelte: '🟠', py: '🐍', rs: '🦀', md: '📝',
			json: '{}', yaml: '📄', yml: '📄', toml: '📄',
			sh: '⌨️', html: '🌐', css: '🎨', sql: '🗄️', png: '🖼', jpg: '🖼', jpeg: '🖼', svg: '🖼'
		};
		return map[ext] ?? '📄';
	}

	function langHint(path: string): string {
		const ext = path.split('.').pop()?.toLowerCase() ?? '';
		const map: Record<string, string> = {
			go: 'go', ts: 'typescript', tsx: 'tsx', js: 'javascript', jsx: 'jsx',
			svelte: 'svelte', py: 'python', rs: 'rust', md: 'markdown',
			json: 'json', yaml: 'yaml', yml: 'yaml', toml: 'toml',
			sh: 'shell', html: 'html', css: 'css', sql: 'sql'
		};
		return map[ext] ?? 'text';
	}
</script>

<svelte:head><title>Fichiers · Hive</title></svelte:head>

<a class="back" href="/projects/{$page.params.id}">← retour au projet</a>

<header class="files-hero">
	<div>
		<h1>Fichiers</h1>
		{#if listing?.root}
			<code class="root">{listing.root}</code>
		{/if}
	</div>
	<div class="hero-stats">
		<span class="stat-chip">{fileCount} fichier{fileCount > 1 ? 's' : ''}</span>
		{#if listing?.truncated}
			<span class="stat-chip warn">liste tronquée</span>
		{/if}
	</div>
</header>

{#if listing?.message}
	<p class="notice">{listing.message}</p>
{/if}
{#if listingError}
	<p class="err">{listingError}</p>
{/if}

<div class="split" class:viewer-active={selected}>
	<nav class="tree" aria-label="arbre des fichiers">
		<div class="tree-search">
			<input type="search"
				placeholder="Rechercher un fichier…"
				bind:value={search} />
			{#if search}
				<span class="search-count">{filteredCount}/{fileCount}</span>
			{/if}
		</div>

		{#if listing && fileCount === 0 && !listing.message}
			<div class="empty">
				<span class="empty-icon">📂</span>
				Workdir vide — l'agent dev n'a rien encore écrit.
			</div>
		{:else if grouped.length === 0}
			<div class="empty">
				<span class="empty-icon">🔍</span>
				Aucun fichier ne matche "{search}".
			</div>
		{:else}
			{#each grouped as [top, files] (top)}
				<details open={search !== '' || top === '' || top === 'src' || top === 'internal'}>
					<summary>
						<span class="sum-icon">{top ? '📁' : '·'}</span>
						<span class="sum-label">{top || 'racine'}</span>
						<span class="sum-count">{files.length}</span>
					</summary>
					<ul>
						{#each files as f (f.path)}
							<li>
								<button
									type="button"
									class:active={selected === f.path}
									onclick={() => pickFile(f.path)}>
									<span class="f-icon">{fileIcon(f.path, f.is_dir)}</span>
									<span class="name">{f.path.slice(top ? top.length + 1 : 0) || f.path}</span>
									<span class="size">{fmtSize(f.size)}</span>
								</button>
							</li>
						{/each}
					</ul>
				</details>
			{/each}
		{/if}
	</nav>

	<section class="viewer">
		{#if !selected}
			<div class="empty viewer-empty">
				<span class="empty-icon">👈</span>
				Sélectionne un fichier dans l'arbre pour voir son contenu.
			</div>
		{:else if contentLoading}
			<div class="empty viewer-empty">
				<span class="empty-icon">⏳</span>
				Chargement…
			</div>
		{:else if contentError}
			<p class="err">{contentError}</p>
		{:else if content}
			<header class="viewer-head">
				<button type="button" class="mobile-back" onclick={clearSelection} aria-label="Retour à l'arbre">
					← arbre
				</button>
				<code class="viewer-path">{content.path}</code>
				<span class="viewer-meta">
					{fmtSize(content.size)}
					{#if content.truncated}· tronqué{/if}
				</span>
				{#if !content.is_binary && content.content}
					<button type="button" class="copy-btn" onclick={copyContent} title="Copier dans le presse-papier">
						📋
					</button>
				{/if}
			</header>
			{#if content.is_binary}
				<div class="notice binary">
					<span class="empty-icon">📦</span>
					Fichier binaire — contenu masqué.
				</div>
			{:else}
				<pre class="code" data-lang={langHint(content.path)}><code>{content.content}</code></pre>
			{/if}
		{/if}
	</section>
</div>

<style>
	.back {
		display: inline-block;
		color: var(--text-muted);
		text-decoration: none;
		font-size: 0.82rem;
		margin-bottom: 0.8rem;
	}
	.back:hover { color: var(--accent); }

	/* ===== Hero ===== */
	.files-hero {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: 1rem;
		flex-wrap: wrap;
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 1rem 1.3rem;
		margin-bottom: 1rem;
	}
	.files-hero h1 {
		margin: 0 0 0.2rem;
		font-size: 1.4rem;
	}
	.root {
		font-family: ui-monospace, monospace;
		font-size: 0.78rem;
		color: var(--text-muted);
		word-break: break-all;
		background: transparent;
		padding: 0;
	}
	.hero-stats {
		display: flex;
		gap: 0.4rem;
		flex-wrap: wrap;
	}
	.stat-chip {
		padding: 0.2rem 0.7rem;
		background: var(--bg-hover);
		border: 1px solid var(--border);
		border-radius: 999px;
		font-size: 0.72rem;
		color: var(--text);
		font-variant-numeric: tabular-nums;
	}
	.stat-chip.warn {
		background: color-mix(in srgb, var(--warn) 14%, transparent);
		border-color: color-mix(in srgb, var(--warn) 40%, transparent);
		color: var(--warn);
	}

	/* ===== Layout ===== */
	.split {
		display: grid;
		grid-template-columns: 320px 1fr;
		gap: 1rem;
		min-height: 500px;
	}

	/* ===== Tree ===== */
	.tree {
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 0.6rem;
		overflow-y: auto;
		max-height: 75vh;
		display: flex;
		flex-direction: column;
	}
	.tree-search {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		padding: 0.4rem;
		margin-bottom: 0.3rem;
		position: sticky;
		top: -0.6rem;
		background: var(--bg-panel);
		z-index: 1;
	}
	.tree-search input {
		flex: 1;
		padding: 0.5rem 0.7rem;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 6px;
		color: inherit;
		font: inherit;
		font-size: 0.82rem;
	}
	.tree-search input:focus {
		outline: none;
		border-color: var(--accent);
	}
	.search-count {
		font-size: 0.7rem;
		color: var(--text-muted);
		font-variant-numeric: tabular-nums;
		padding: 0 0.3rem;
	}
	.tree details { margin-bottom: 0.2rem; }
	.tree summary {
		list-style: none;
		cursor: pointer;
		display: flex;
		align-items: center;
		gap: 0.4rem;
		padding: 0.4rem 0.5rem;
		border-radius: 6px;
		font-size: 0.78rem;
		color: var(--text);
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		user-select: none;
	}
	.tree summary::-webkit-details-marker { display: none; }
	.tree summary:hover { background: var(--bg-hover); }
	.tree details[open] summary { background: var(--bg-hover); }
	.sum-icon { font-size: 0.85rem; }
	.sum-label { flex: 1; }
	.sum-count {
		font-size: 0.7rem;
		color: var(--text-muted);
		background: var(--bg);
		padding: 1px 8px;
		border-radius: 10px;
		font-weight: 500;
		text-transform: none;
		letter-spacing: 0;
	}
	.tree ul {
		list-style: none;
		padding: 0.25rem 0 0.35rem 0.75rem;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 0.05rem;
	}
	.tree button {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		width: 100%;
		padding: 0.4rem 0.55rem;
		background: transparent;
		border: none;
		border-radius: 5px;
		cursor: pointer;
		color: inherit;
		font: inherit;
		font-size: 0.82rem;
		text-align: left;
		min-height: auto;
	}
	.tree button:hover { background: var(--bg-hover); }
	.tree button.active {
		background: color-mix(in srgb, var(--accent) 18%, transparent);
		color: var(--accent);
	}
	.f-icon {
		font-size: 0.85rem;
		width: 1.1rem;
		text-align: center;
		flex-shrink: 0;
	}
	.tree .name {
		flex: 1;
		font-family: ui-monospace, monospace;
		font-size: 0.8rem;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		min-width: 0;
	}
	.tree .size {
		color: var(--text-muted);
		font-size: 0.68rem;
		font-variant-numeric: tabular-nums;
		flex-shrink: 0;
	}

	/* ===== Viewer ===== */
	.viewer {
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 10px;
		display: flex;
		flex-direction: column;
		overflow: hidden;
		min-width: 0;
	}
	.viewer-head {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 0.5rem;
		padding: 0.6rem 0.85rem;
		background: var(--bg-hover);
		border-bottom: 1px solid var(--border);
	}
	.mobile-back {
		display: none;
		background: transparent;
		border: none;
		color: var(--accent);
		font: inherit;
		font-size: 0.82rem;
		cursor: pointer;
		padding: 0.3rem 0.5rem;
	}
	.viewer-path {
		flex: 1;
		min-width: 0;
		font-family: ui-monospace, monospace;
		font-size: 0.82rem;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		background: transparent;
		padding: 0;
	}
	.viewer-meta {
		font-size: 0.72rem;
		color: var(--text-muted);
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
	}
	.copy-btn {
		background: transparent;
		border: 1px solid var(--border);
		border-radius: 4px;
		padding: 0.3rem 0.5rem;
		cursor: pointer;
		font-size: 0.85rem;
		min-height: auto;
	}
	.copy-btn:hover { border-color: var(--accent); }
	.code {
		margin: 0;
		padding: 1rem 1.2rem;
		background: var(--bg);
		color: var(--text);
		font-family: ui-monospace, monospace;
		font-size: 0.82rem;
		line-height: 1.55;
		overflow: auto;
		max-height: 75vh;
		white-space: pre;
	}

	/* ===== Empty / notices ===== */
	.empty {
		text-align: center;
		padding: 2rem 1rem;
		color: var(--text-muted);
		font-style: italic;
	}
	.viewer-empty { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; }
	.empty-icon {
		display: block;
		font-size: 2rem;
		margin-bottom: 0.5rem;
		font-style: normal;
		opacity: 0.5;
	}
	.notice {
		padding: 0.7rem 0.9rem;
		background: color-mix(in srgb, var(--accent) 12%, transparent);
		border-left: 3px solid var(--accent);
		border-radius: 0 6px 6px 0;
		font-size: 0.85rem;
		margin-bottom: 1rem;
	}
	.notice.binary {
		margin: 1.5rem;
		text-align: center;
		border-left: 0;
		background: var(--bg);
		border: 1px dashed var(--border);
		border-radius: 8px;
	}
	.err {
		padding: 0.7rem 0.9rem;
		background: color-mix(in srgb, var(--err) 12%, transparent);
		border-left: 3px solid var(--err);
		border-radius: 0 6px 6px 0;
		color: var(--err);
		font-size: 0.85rem;
	}

	/* ===== Responsive ===== */
	@media (max-width: 767px) {
		.split {
			grid-template-columns: 1fr;
			gap: 0;
			min-height: 0;
		}
		/* Default : tree visible, viewer hidden. Quand un fichier est
		   sélectionné (viewer-active), on swap. */
		.split .viewer { display: none; }
		.split.viewer-active .tree { display: none; }
		.split.viewer-active .viewer { display: flex; }
		.tree { max-height: 70vh; }
		.mobile-back { display: inline-block; }
	}
</style>

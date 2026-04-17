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

	$effect(() => {
		loadList();
		// Any story event = Claude Code may have just written something new;
		// refresh the tree. Don't thrash the content pane — user may be reading.
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

	// Group files by top-level directory for a compact tree. Not a real
	// recursive tree — flat with prefix grouping is good enough and
	// keeps rendering cheap for a few-thousand-file codebase.
	let grouped = $derived.by(() => {
		const groups = new Map<string, FileEntry[]>();
		for (const f of listing?.files ?? []) {
			const i = f.path.indexOf('/');
			const top = i < 0 ? '' : f.path.slice(0, i);
			if (!groups.has(top)) groups.set(top, []);
			groups.get(top)!.push(f);
		}
		return [...groups.entries()].sort(([a], [b]) => a.localeCompare(b));
	});

	function fmtSize(n: number): string {
		if (n < 1024) return `${n} B`;
		if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
		return `${(n / (1024 * 1024)).toFixed(1)} MB`;
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

<main>
	<a class="back" href="/projects/{$page.params.id}">← back to project</a>
	<header>
		<h1>Files</h1>
		{#if listing?.root}
			<code class="root">{listing.root}</code>
		{/if}
	</header>

	{#if listing?.message}
		<p class="notice">{listing.message}</p>
	{/if}
	{#if listingError}
		<p class="err">{listingError}</p>
	{/if}
	{#if listing?.truncated}
		<p class="notice">List capped — showing first {listing.files.length} entries.</p>
	{/if}

	<div class="split">
		<nav class="tree" aria-label="file tree">
			{#if listing && listing.files.length === 0 && !listing.message}
				<p class="empty">Workdir is empty — the dev agent hasn't written anything yet.</p>
			{/if}
			{#each grouped as [top, files] (top)}
				<details open={top === '' || top === 'src'}>
					<summary>{top || '/'}</summary>
					<ul>
						{#each files as f (f.path)}
							<li>
								<button
									type="button"
									class:active={selected === f.path}
									onclick={() => pickFile(f.path)}
								>
									<span class="name">{f.path.slice(top ? top.length + 1 : 0) || f.path}</span>
									<span class="size">{fmtSize(f.size)}</span>
								</button>
							</li>
						{/each}
					</ul>
				</details>
			{/each}
		</nav>

		<section class="viewer">
			{#if !selected}
				<p class="empty">Pick a file to view its contents.</p>
			{:else if contentLoading}
				<p class="empty">Loading…</p>
			{:else if contentError}
				<p class="err">{contentError}</p>
			{:else if content}
				<header class="viewer-head">
					<code>{content.path}</code>
					<span class="muted">{fmtSize(content.size)}{content.truncated ? ' · truncated' : ''}</span>
				</header>
				{#if content.is_binary}
					<p class="notice">Binary file — content hidden.</p>
				{:else}
					<pre class="code" data-lang={langHint(content.path)}><code>{content.content}</code></pre>
				{/if}
			{/if}
		</section>
	</div>
</main>

<style>
	main {
		display: flex;
		flex-direction: column;
		gap: 1rem;
		max-width: 1400px;
	}
	.back {
		color: var(--muted);
		text-decoration: none;
		font-size: 0.85rem;
	}
	.back:hover { color: var(--accent); }
	header {
		display: flex;
		align-items: baseline;
		gap: 0.75rem;
	}
	h1 { margin: 0; }
	.root {
		font-size: 0.75rem;
		color: var(--muted);
		font-family: ui-monospace, monospace;
	}
	.notice {
		padding: 0.5rem 0.75rem;
		background: color-mix(in srgb, var(--accent) 12%, var(--bg-alt));
		border-left: 3px solid var(--accent);
		border-radius: 4px;
		font-size: 0.85rem;
	}
	.err {
		padding: 0.5rem 0.75rem;
		background: rgba(240, 80, 80, 0.15);
		border-left: 3px solid var(--err);
		border-radius: 4px;
		color: var(--err);
		font-size: 0.85rem;
	}
	.empty {
		color: var(--muted);
		font-style: italic;
		padding: 1rem;
	}
	.split {
		display: grid;
		grid-template-columns: 320px 1fr;
		gap: 1rem;
		min-height: 500px;
	}
	.tree {
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 0.5rem;
		overflow-y: auto;
		max-height: 75vh;
	}
	.tree details { margin-bottom: 0.35rem; }
	.tree summary {
		cursor: pointer;
		font-size: 0.8rem;
		font-weight: 600;
		color: var(--muted);
		padding: 0.2rem 0.3rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}
	.tree ul {
		list-style: none;
		padding: 0 0 0 0.5rem;
		margin: 0.25rem 0 0.5rem;
		display: flex;
		flex-direction: column;
		gap: 0.1rem;
	}
	.tree button {
		display: flex;
		justify-content: space-between;
		gap: 0.5rem;
		align-items: baseline;
		width: 100%;
		padding: 0.25rem 0.45rem;
		background: transparent;
		border: none;
		border-radius: 3px;
		cursor: pointer;
		color: inherit;
		font: inherit;
		font-size: 0.82rem;
		text-align: left;
	}
	.tree button:hover { background: var(--bg); }
	.tree button.active {
		background: color-mix(in srgb, var(--accent) 22%, var(--bg));
		color: var(--text);
	}
	.tree .name {
		font-family: ui-monospace, monospace;
		font-size: 0.8rem;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.tree .size {
		color: var(--muted);
		font-size: 0.7rem;
	}
	.viewer {
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 6px;
		display: flex;
		flex-direction: column;
		overflow: hidden;
	}
	.viewer-head {
		display: flex;
		justify-content: space-between;
		align-items: baseline;
		padding: 0.5rem 0.75rem;
		background: var(--bg);
		border-bottom: 1px solid var(--border);
		font-size: 0.8rem;
	}
	.viewer-head code {
		font-family: ui-monospace, monospace;
		font-size: 0.82rem;
	}
	.muted { color: var(--muted); font-size: 0.75rem; }
	.code {
		margin: 0;
		padding: 0.75rem 1rem;
		background: var(--bg);
		color: var(--text);
		font-family: ui-monospace, monospace;
		font-size: 0.8rem;
		line-height: 1.5;
		overflow: auto;
		max-height: 75vh;
		white-space: pre;
	}
</style>

<script lang="ts">
	import { page } from '$app/stores';
	import { apiGet, apiPost } from '$lib/api';
	import { fmtRelative } from '$lib/format';

	type Message = { id: number; author: string; content: string; created_at: string };
	type Conversation = {
		id: string;
		project_id: string;
		role: string;
		status: string;
		messages?: Message[];
	};
	type Project = { id: string; name: string; idea: string; status: string };

	let project = $state<Project | null>(null);
	let conversation = $state<Conversation | null>(null);
	let replyDraft = $state('');
	let sending = $state(false);
	let finalizing = $state(false);
	let done = $state(false);
	let error = $state('');

	async function loadProject() {
		const id = $page.params.id ?? '';
		try {
			project = await apiGet<Project>(`/api/v1/projects/${encodeURIComponent(id)}`);
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
		}
	}

	async function loadConv() {
		const id = $page.params.id ?? '';
		try {
			conversation = await apiGet<Conversation>(`/api/v1/projects/${encodeURIComponent(id)}/iterate`);
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
		}
	}

	async function sendReply() {
		const id = $page.params.id ?? '';
		if (!replyDraft.trim()) return;
		error = '';
		sending = true;
		try {
			const resp = (await apiPost(
				`/api/v1/projects/${encodeURIComponent(id)}/iterate/messages`,
				{ content: replyDraft }
			)) as { conversation: Conversation; done: boolean };
			conversation = resp.conversation;
			done = resp.done;
			replyDraft = '';
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
		} finally {
			sending = false;
		}
	}

	async function finalize() {
		const id = $page.params.id ?? '';
		finalizing = true;
		error = '';
		try {
			await apiPost(`/api/v1/projects/${encodeURIComponent(id)}/iterate/finalize`, {});
			window.location.href = `/projects/${encodeURIComponent(id)}`;
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
			finalizing = false;
		}
	}

	function onKeydown(e: KeyboardEvent) {
		// Cmd/Ctrl + Enter pour envoyer — raccourci standard.
		if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
			e.preventDefault();
			sendReply();
		}
	}

	$effect(() => {
		loadProject();
		loadConv();
	});
</script>

<svelte:head><title>Itération · {project?.name ?? 'Hive'}</title></svelte:head>

<a class="back" href="/projects/{$page.params.id}">← retour au projet</a>

{#if project}
	<header class="iter-hero">
		<span class="chip brownfield">➕ Nouvelle itération</span>
		<h1>{project.name}</h1>
		<p class="hero-idea">{project.idea}</p>
		<p class="hero-help">
			Décris à l'agent PM la feature ou la correction que tu veux ajouter. Il te
			posera quelques questions pour cadrer, puis étendra le PRD et relancera
			BMAD en mode brownfield (<code>/bmad-document-project</code> → <code>/bmad-edit-prd</code>).
		</p>
	</header>
{/if}

{#if error}
	<div class="err">{error}</div>
{/if}

{#if conversation}
	<section class="chat-panel">
		<div class="chat">
			{#each conversation.messages ?? [] as m (m.id)}
				<div class="bubble" class:user={m.author === 'user'} class:agent={m.author !== 'user'}>
					<div class="bubble-head">
						<span class="bubble-avatar" class:user={m.author === 'user'}>
							{m.author === 'user' ? '👤' : '🤖'}
						</span>
						<strong>{m.author === 'user' ? 'Toi' : 'Agent PM — itération'}</strong>
						<span class="bubble-time">{fmtRelative(m.created_at)}</span>
					</div>
					<div class="bubble-content">{m.content}</div>
				</div>
			{/each}
			{#if (conversation.messages ?? []).length === 0}
				<div class="empty-chat">
					<span class="empty-icon">💬</span>
					L'agent PM va démarrer la conversation. Envoie ton premier message ci-dessous.
				</div>
			{/if}
		</div>

		<form class="composer" onsubmit={(e) => { e.preventDefault(); sendReply(); }}>
			<textarea
				rows="3"
				bind:value={replyDraft}
				onkeydown={onKeydown}
				placeholder={done ? 'Ajoute un détail ou finalise à droite →' : 'Décris la feature à ajouter…'}
				disabled={sending || finalizing}
			></textarea>
			<div class="composer-foot">
				<span class="hint">
					<kbd>⌘</kbd><kbd>↵</kbd> pour envoyer
				</span>
				<div class="composer-actions">
					<button type="submit"
						class="btn"
						disabled={sending || finalizing || !replyDraft.trim()}>
						{sending ? 'Envoi…' : 'Envoyer'}
					</button>
					{#if done}
						<button type="button"
							class="btn primary"
							onclick={finalize}
							disabled={finalizing}>
							{finalizing ? '⏳ Lancement BMAD…' : '✓ Finaliser & lancer'}
						</button>
					{/if}
				</div>
			</div>
			{#if done}
				<p class="done-note">
					✨ L'agent a assez d'info. Clique <strong>Finaliser & lancer</strong> pour
					déclencher l'IterationPipeline BMAD.
				</p>
			{/if}
		</form>
	</section>
{:else}
	<div class="empty">
		<span class="empty-icon">⏳</span>
		Préparation de la conversation d'itération…
	</div>
{/if}

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
	.iter-hero {
		background:
			radial-gradient(ellipse at top right, color-mix(in srgb, var(--accent) 10%, transparent), transparent 60%),
			var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 1.3rem 1.5rem;
		margin-bottom: 1rem;
	}
	.chip {
		display: inline-flex;
		align-items: center;
		gap: 0.3rem;
		padding: 0.2rem 0.7rem;
		background: color-mix(in srgb, var(--accent) 14%, transparent);
		border: 1px solid color-mix(in srgb, var(--accent) 40%, transparent);
		border-radius: 999px;
		font-size: 0.7rem;
		color: var(--accent);
		font-weight: 700;
		letter-spacing: 0.04em;
		margin-bottom: 0.7rem;
	}
	.iter-hero h1 {
		font-size: 1.6rem;
		margin: 0 0 0.4rem;
		letter-spacing: -0.01em;
	}
	.hero-idea {
		margin: 0 0 0.7rem;
		color: var(--text);
		font-size: 0.95rem;
		font-weight: 500;
	}
	.hero-help {
		margin: 0;
		color: var(--text-muted);
		font-size: 0.85rem;
		line-height: 1.55;
	}
	.hero-help code {
		background: var(--bg-hover);
		padding: 1px 6px;
		border-radius: 3px;
		font-size: 0.78rem;
		color: var(--accent);
	}

	/* ===== Chat panel ===== */
	.chat-panel {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}
	.chat {
		display: flex;
		flex-direction: column;
		gap: 0.8rem;
		max-height: 520px;
		overflow-y: auto;
		padding: 1rem;
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 10px;
	}
	.bubble {
		max-width: 80%;
		padding: 0.7rem 0.9rem;
		border-radius: 12px;
		font-size: 0.92rem;
		line-height: 1.5;
	}
	.bubble.user {
		align-self: flex-end;
		background: var(--accent);
		color: white;
		border-bottom-right-radius: 4px;
	}
	.bubble.agent {
		align-self: flex-start;
		background: var(--bg);
		border: 1px solid var(--border);
		border-bottom-left-radius: 4px;
	}
	.bubble-head {
		display: flex;
		gap: 0.4rem;
		align-items: center;
		font-size: 0.7rem;
		opacity: 0.75;
		margin-bottom: 0.35rem;
	}
	.bubble.user .bubble-head { color: rgba(255, 255, 255, 0.85); }
	.bubble.agent .bubble-head { color: var(--text-muted); }
	.bubble-avatar {
		font-size: 0.8rem;
		opacity: 0.8;
	}
	.bubble-time {
		margin-left: auto;
		font-variant-numeric: tabular-nums;
	}
	.bubble-content {
		white-space: pre-wrap;
		word-break: break-word;
	}
	.empty-chat {
		text-align: center;
		padding: 2rem 1rem;
		color: var(--text-muted);
		font-style: italic;
	}
	.empty-icon {
		display: block;
		font-size: 2rem;
		margin-bottom: 0.5rem;
		font-style: normal;
		opacity: 0.5;
	}

	/* ===== Composer ===== */
	.composer {
		display: flex;
		flex-direction: column;
		gap: 0.6rem;
		padding: 0.9rem;
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 10px;
	}
	.composer textarea {
		padding: 0.7rem 0.85rem;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 8px;
		color: inherit;
		font: inherit;
		font-size: 0.92rem;
		resize: vertical;
		min-height: 80px;
		line-height: 1.5;
	}
	.composer textarea:focus {
		outline: none;
		border-color: var(--accent);
	}
	.composer-foot {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 0.5rem;
		flex-wrap: wrap;
	}
	.hint {
		font-size: 0.72rem;
		color: var(--text-muted);
	}
	.hint kbd {
		display: inline-block;
		padding: 1px 5px;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 3px;
		font-family: ui-monospace, monospace;
		font-size: 0.7rem;
		margin: 0 1px;
	}
	.composer-actions {
		display: flex;
		gap: 0.5rem;
	}
	.btn {
		padding: 0.55rem 1rem;
		border-radius: 6px;
		font-weight: 600;
		font-size: 0.85rem;
		cursor: pointer;
		border: 1px solid var(--border);
		background: var(--bg);
		color: var(--text);
	}
	.btn:hover { border-color: var(--accent); color: var(--accent); }
	.btn.primary {
		background: var(--accent);
		color: white;
		border-color: var(--accent);
	}
	.btn.primary:hover { background: color-mix(in srgb, var(--accent) 88%, black); color: white; }
	.btn:disabled { opacity: 0.5; cursor: not-allowed; }

	.done-note {
		margin: 0;
		padding: 0.7rem 0.85rem;
		background: color-mix(in srgb, var(--ok) 12%, transparent);
		border-left: 3px solid var(--ok);
		border-radius: 0 6px 6px 0;
		font-size: 0.85rem;
		color: var(--text);
	}
	.done-note strong { color: var(--ok); }

	.err {
		padding: 0.7rem 0.9rem;
		background: color-mix(in srgb, var(--err) 12%, transparent);
		border-left: 3px solid var(--err);
		border-radius: 0 6px 6px 0;
		color: var(--err);
		font-size: 0.85rem;
		margin-bottom: 1rem;
	}
	.empty {
		padding: 2rem 1rem;
		text-align: center;
		color: var(--text-muted);
		background: var(--bg-panel);
		border: 1px dashed var(--border);
		border-radius: 8px;
		font-style: italic;
	}

	/* ===== Responsive ===== */
	@media (max-width: 767px) {
		.iter-hero { padding: 1rem 1.1rem; }
		.iter-hero h1 { font-size: 1.3rem; }
		.bubble { max-width: 92%; font-size: 0.88rem; }
		.chat { max-height: 60vh; padding: 0.7rem; }
		.hint { display: none; }
	}
</style>

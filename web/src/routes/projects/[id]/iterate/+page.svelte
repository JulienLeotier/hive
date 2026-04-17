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

	$effect(() => {
		loadProject();
		loadConv();
	});
</script>

<main>
	<a class="back" href="/projects/{$page.params.id}">← retour au projet</a>

	{#if project}
		<header>
			<h1>Nouvelle itération</h1>
			<p class="muted">Projet : <strong>{project.name}</strong> — {project.idea}</p>
		</header>
	{/if}

	{#if error}<div class="err">{error}</div>{/if}

	{#if conversation}
		<section class="chat">
			{#each conversation.messages ?? [] as m (m.id)}
				<div class="bubble" class:user={m.author === 'user'} class:agent={m.author !== 'user'}>
					<div class="bubble-head">
						<strong>{m.author === 'user' ? 'Toi' : 'Agent PM (itération)'}</strong>
						<span class="muted">{fmtRelative(m.created_at)}</span>
					</div>
					<div class="bubble-content">{m.content}</div>
				</div>
			{/each}
		</section>

		<form class="reply-form" onsubmit={(e) => { e.preventDefault(); sendReply(); }}>
			<textarea
				rows="3"
				bind:value={replyDraft}
				placeholder="Décris la feature à ajouter…"
				disabled={sending || finalizing}
			></textarea>
			<div class="actions">
				<button type="submit" disabled={sending || finalizing || !replyDraft.trim()}>
					{sending ? 'Envoi…' : 'Envoyer'}
				</button>
				{#if done}
					<button type="button" class="primary" onclick={finalize} disabled={finalizing}>
						{finalizing ? 'Lancement BMAD…' : '✓ Finaliser et lancer l\'itération'}
					</button>
					<span class="muted">L'agent a assez d'info pour étendre le PRD.</span>
				{/if}
			</div>
		</form>
	{:else}
		<p class="empty">Chargement…</p>
	{/if}
</main>

<style>
	main { display: flex; flex-direction: column; gap: 1rem; max-width: 1000px; }
	.back { color: var(--muted); text-decoration: none; font-size: 0.85rem; }
	.back:hover { color: var(--accent); }
	h1 { margin: 0 0 0.25rem; }
	.muted { color: var(--muted); }
	.chat { display: flex; flex-direction: column; gap: 0.6rem; max-height: 520px; overflow-y: auto; padding: 0.5rem; background: var(--bg-alt); border: 1px solid var(--border); border-radius: 6px; }
	.bubble { padding: 0.55rem 0.75rem; border-radius: 8px; max-width: 75%; }
	.bubble.user { align-self: flex-end; background: color-mix(in srgb, var(--accent) 18%, var(--bg)); border: 1px solid color-mix(in srgb, var(--accent) 40%, var(--border)); }
	.bubble.agent { align-self: flex-start; background: var(--bg); border: 1px solid var(--border); }
	.bubble-head { display: flex; gap: 0.5rem; font-size: 0.7rem; color: var(--muted); margin-bottom: 0.25rem; }
	.bubble-content { white-space: pre-wrap; font-size: 0.9rem; line-height: 1.4; }
	.reply-form { display: flex; flex-direction: column; gap: 0.5rem; }
	.reply-form textarea { padding: 0.55rem 0.75rem; background: var(--bg-alt); border: 1px solid var(--border); border-radius: 6px; color: inherit; font: inherit; resize: vertical; }
	.actions { display: flex; gap: 0.5rem; align-items: center; flex-wrap: wrap; }
	.actions button { padding: 0.45rem 0.85rem; background: var(--bg-alt); border: 1px solid var(--border); border-radius: 6px; cursor: pointer; color: inherit; font-weight: 500; }
	.actions button.primary { background: var(--accent); color: white; border: none; }
	.actions button:disabled { opacity: 0.5; cursor: not-allowed; }
	.err { padding: 0.5rem 0.75rem; background: rgba(240, 80, 80, 0.15); border-left: 3px solid var(--err); border-radius: 4px; color: var(--err); font-size: 0.85rem; }
	.empty { color: var(--muted); font-style: italic; }
</style>

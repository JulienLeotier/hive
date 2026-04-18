<script lang="ts">
	// ConfirmHost : le composant qui rend la modal de confirmation. À
	// monter une seule fois dans +layout.svelte. Écoute le store
	// confirmState et affiche / cache la modal. Gère Esc, click
	// backdrop, focus trap basique, Enter = confirmer.

	import { onMount } from 'svelte';
	import { confirmState, closeConfirm } from './confirm';

	let confirmBtn: HTMLButtonElement | undefined = $state(undefined);

	// Focus le bouton Confirmer dès qu'une modal s'ouvre pour que
	// Enter la valide (raccourci clavier par défaut).
	$effect(() => {
		if ($confirmState && confirmBtn) {
			queueMicrotask(() => confirmBtn?.focus());
		}
	});

	function onKeydown(e: KeyboardEvent) {
		if (!$confirmState) return;
		if (e.key === 'Escape') {
			e.preventDefault();
			closeConfirm(false);
		} else if (e.key === 'Enter') {
			e.preventDefault();
			closeConfirm(true);
		}
	}

	onMount(() => {
		window.addEventListener('keydown', onKeydown);
		return () => window.removeEventListener('keydown', onKeydown);
	});
</script>

{#if $confirmState}
	{@const s = $confirmState}
	<div
		class="backdrop"
		role="presentation"
		onclick={() => closeConfirm(false)}
	></div>
	<div
		class="modal"
		role="alertdialog"
		aria-modal="true"
		aria-labelledby="confirm-title"
		aria-describedby="confirm-message"
	>
		{#if s.title}
			<h2 id="confirm-title" class="title">{s.title}</h2>
		{/if}
		<p id="confirm-message" class="message">{s.message}</p>
		<div class="actions">
			<button type="button" class="btn ghost" onclick={() => closeConfirm(false)}>
				{s.cancelLabel ?? 'Annuler'}
			</button>
			<button
				bind:this={confirmBtn}
				type="button"
				class="btn primary"
				class:danger={s.danger}
				onclick={() => closeConfirm(true)}
			>
				{s.confirmLabel ?? 'Confirmer'}
			</button>
		</div>
	</div>
{/if}

<style>
	.backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.45);
		z-index: 400;
		animation: fade-in 0.15s ease-out;
	}
	.modal {
		position: fixed;
		top: 50%;
		left: 50%;
		transform: translate(-50%, -50%);
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 12px;
		padding: 1.4rem 1.5rem 1.25rem;
		min-width: 320px;
		max-width: min(520px, 92vw);
		box-shadow: 0 24px 60px rgba(0, 0, 0, 0.35);
		z-index: 401;
		animation: pop-in 0.18s ease-out;
	}
	.title {
		margin: 0 0 0.6rem;
		font-size: 1.05rem;
		font-weight: 700;
		color: var(--text);
		line-height: 1.3;
	}
	.message {
		margin: 0 0 1.2rem;
		font-size: 0.9rem;
		line-height: 1.5;
		color: var(--text);
		white-space: pre-line;
	}
	.actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
	}
	.btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 0.5rem 0.9rem;
		border-radius: 6px;
		font-size: 0.85rem;
		font-weight: 600;
		border: 1px solid var(--border);
		background: var(--bg-panel);
		color: var(--text);
		cursor: pointer;
		font-family: inherit;
		line-height: 1.25;
		transition: border-color 0.1s, background 0.1s, color 0.1s;
	}
	.btn.ghost:hover { border-color: var(--accent); color: var(--accent); }
	.btn.primary {
		background: var(--accent);
		border-color: var(--accent);
		color: #fff;
	}
	.btn.primary:hover { opacity: 0.9; }
	.btn.primary.danger {
		background: var(--err, #dc2626);
		border-color: var(--err, #dc2626);
	}
	.btn:focus-visible {
		outline: 2px solid color-mix(in srgb, var(--accent) 40%, transparent);
		outline-offset: 2px;
	}
	@keyframes fade-in {
		from { opacity: 0; }
		to { opacity: 1; }
	}
	@keyframes pop-in {
		from { opacity: 0; transform: translate(-50%, -48%) scale(0.96); }
		to { opacity: 1; transform: translate(-50%, -50%) scale(1); }
	}
</style>

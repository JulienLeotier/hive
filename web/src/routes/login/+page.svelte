<script lang="ts">
	import { goto } from '$app/navigation';
	import { setStoredKey } from '$lib/api';

	let key = $state('');
	let error = $state('');
	let submitting = $state(false);

	async function submit(ev: Event) {
		ev.preventDefault();
		error = '';
		submitting = true;
		try {
			// Probe: send the key against an authenticated endpoint. If the
			// server accepts it, persist and redirect. No point storing a
			// bad key and letting the dashboard 401-loop.
			const r = await fetch('/api/v1/metrics', {
				headers: { Authorization: `Bearer ${key.trim()}` }
			});
			if (r.status === 401) {
				error = 'That key was rejected. Double-check it was copied in full.';
				return;
			}
			if (!r.ok) {
				error = `Server responded ${r.status}. Try again.`;
				return;
			}
			setStoredKey(key.trim());
			goto('/');
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
		} finally {
			submitting = false;
		}
	}
</script>

<main>
	<h1>Sign in</h1>
	<p class="lead">Paste your Hive API key to access the dashboard.</p>

	<form onsubmit={submit}>
		<label>
			API key
			<input
				type="password"
				placeholder="hive_…"
				bind:value={key}
				required
				autocomplete="current-password"
			/>
			<small>
				Keys look like <code>hive_</code> followed by 64 hex characters. Lost yours?
				Mint a new one with <code>hive api-key create &lt;name&gt;</code>.
			</small>
		</label>

		<button type="submit" disabled={submitting || key.length === 0}>
			{submitting ? 'Verifying…' : 'Sign in'}
		</button>

		{#if error}<div class="error">{error}</div>{/if}
	</form>
</main>

<style>
	main {
		max-width: 480px;
		margin: 4rem auto;
		padding: 2rem;
		font-family: system-ui, sans-serif;
	}
	h1 { margin-top: 0; }
	.lead { color: var(--muted); margin-bottom: 2rem; }
	form { display: flex; flex-direction: column; gap: 1.25rem; }
	label { display: flex; flex-direction: column; gap: 0.35rem; font-size: 0.9rem; color: var(--muted); }
	input {
		padding: 0.55rem 0.75rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 6px;
		color: inherit;
		font: inherit;
		font-family: ui-monospace, monospace;
	}
	small { font-size: 0.8rem; color: var(--muted); }
	button {
		padding: 0.65rem 1rem;
		background: var(--accent);
		color: white;
		border: none;
		border-radius: 6px;
		cursor: pointer;
		font-weight: 600;
	}
	button:disabled { opacity: 0.5; cursor: not-allowed; }
	.error {
		padding: 0.6rem 0.85rem;
		background: rgba(240, 80, 80, 0.15);
		border-left: 3px solid var(--err);
		border-radius: 4px;
		color: var(--err);
	}
</style>

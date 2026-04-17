<script lang="ts">
	import { goto } from '$app/navigation';
	import { apiPost, setStoredKey } from '$lib/api';

	let subject = $state('');
	let tenant = $state('default');
	let submitting = $state(false);
	let error = $state('');
	let result = $state<{ subject: string; api_key: string } | null>(null);

	async function bootstrap(ev: Event) {
		ev.preventDefault();
		error = '';
		submitting = true;
		try {
			result = (await apiPost('/api/v1/setup/bootstrap', {
				subject,
				tenant_id: tenant
			})) as { subject: string; api_key: string };
			// Persist so the dashboard can immediately issue authenticated
			// requests without the user re-typing the key. They still see it
			// once for their password manager.
			setStoredKey(result.api_key);
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
		} finally {
			submitting = false;
		}
	}

	function continueToDashboard() {
		goto('/');
	}

	function copy(text: string) {
		navigator.clipboard.writeText(text);
	}
</script>

<main>
	<h1>Welcome to Hive</h1>
	<p class="lead">No admin user is configured yet. Create one to start using the dashboard.</p>

	{#if !result}
		<form onsubmit={bootstrap}>
			<label>
				Admin identity
				<input
					type="text"
					placeholder="alice@example.com"
					bind:value={subject}
					required
					autocomplete="username"
				/>
				<small>Any string — typically an email. You'll present this as the API key's owner.</small>
			</label>

			<label>
				Tenant
				<input type="text" bind:value={tenant} required />
				<small>Tenant namespace this admin belongs to. "default" is fine for a single-team deployment.</small>
			</label>

			<button type="submit" disabled={submitting}>
				{submitting ? 'Creating…' : 'Create admin and bootstrap API key'}
			</button>

			{#if error}
				<div class="error">{error}</div>
			{/if}
		</form>
	{:else}
		<div class="result">
			<h2>✓ Admin created</h2>
			<p>
				Copy the API key below — this is the only time it's shown. Store it in your
				password manager, then pass it on requests via
				<code>Authorization: Bearer &lt;key&gt;</code>.
			</p>
			<div class="secret">
				<code>{result.api_key}</code>
				<button class="copy" onclick={() => copy(result!.api_key)}>Copy</button>
			</div>
			<button class="next" onclick={continueToDashboard}>Go to the dashboard →</button>
		</div>
	{/if}
</main>

<style>
	main {
		max-width: 560px;
		margin: 4rem auto;
		padding: 2rem;
		font-family: system-ui, sans-serif;
	}
	h1 {
		margin-top: 0;
	}
	.lead {
		color: var(--muted);
		margin-bottom: 2rem;
	}
	form {
		display: flex;
		flex-direction: column;
		gap: 1.25rem;
	}
	label {
		display: flex;
		flex-direction: column;
		gap: 0.35rem;
		font-size: 0.9rem;
		color: var(--muted);
	}
	input {
		padding: 0.55rem 0.75rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 6px;
		color: inherit;
		font: inherit;
	}
	small {
		font-size: 0.8rem;
		color: var(--muted);
	}
	button {
		padding: 0.65rem 1rem;
		background: var(--accent);
		color: white;
		border: none;
		border-radius: 6px;
		cursor: pointer;
		font-weight: 600;
	}
	button:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.error {
		padding: 0.6rem 0.85rem;
		background: rgba(240, 80, 80, 0.15);
		border-left: 3px solid var(--err);
		border-radius: 4px;
		color: var(--err);
	}
	.result {
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}
	.secret {
		display: flex;
		gap: 0.5rem;
		align-items: center;
		padding: 0.75rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 6px;
		overflow-x: auto;
	}
	.secret code {
		flex: 1;
		font-family: ui-monospace, monospace;
		font-size: 0.85rem;
		white-space: nowrap;
	}
	.copy {
		padding: 0.3rem 0.7rem;
		font-size: 0.85rem;
		background: transparent;
		color: var(--fg);
		border: 1px solid var(--border);
	}
	.copy:hover {
		background: var(--bg-alt);
	}
	.next {
		align-self: flex-start;
	}
</style>

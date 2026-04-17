<script lang="ts">
	import { apiGet, apiPost, apiDelete } from '$lib/api';
	import type { User } from '$lib/types';
	import ListScaffold from '$lib/ListScaffold.svelte';

	let users = $state<User[]>([]);
	let loading = $state(true);

	let newSubject = $state('');
	let newRole = $state('viewer');
	let newTenant = $state('default');
	let formError = $state('');
	let submitting = $state(false);

	async function load() {
		try {
			users = (await apiGet<User[]>('/api/v1/users')) ?? [];
		} catch {
			/* banner shown by apiGet */
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		load();
	});

	async function createUser(ev: Event) {
		ev.preventDefault();
		formError = '';
		submitting = true;
		try {
			await apiPost('/api/v1/users', {
				subject: newSubject,
				role: newRole,
				tenant_id: newTenant
			});
			newSubject = '';
			await load();
		} catch (e) {
			formError = e instanceof Error ? e.message : String(e);
		} finally {
			submitting = false;
		}
	}

	async function removeUser(subject: string) {
		if (!confirm(`Remove user "${subject}"? API keys remain but resolve to viewer.`)) return;
		try {
			await apiDelete(`/api/v1/users/${encodeURIComponent(subject)}`);
			await load();
		} catch (e) {
			formError = e instanceof Error ? e.message : String(e);
		}
	}

	function roleColor(r: string): string {
		if (r === 'admin') return 'var(--err)';
		if (r === 'operator') return 'var(--accent)';
		return 'var(--text-muted)';
	}
</script>

<ListScaffold
	title="Users"
	subtitle="RBAC directory — OIDC subject → role + tenant mapping. Tenants are implicit; entering a new tenant here creates it."
	{loading}
	isEmpty={users.length === 0}
	emptyText="No users registered. Create one with the form below."
>
	<form class="create-form" onsubmit={createUser}>
		<input placeholder="subject (email or OIDC sub)" bind:value={newSubject} required />
		<select bind:value={newRole}>
			<option value="viewer">viewer</option>
			<option value="operator">operator</option>
			<option value="admin">admin</option>
		</select>
		<input placeholder="tenant_id" bind:value={newTenant} required />
		<button type="submit" disabled={submitting}>{submitting ? '…' : 'Add user'}</button>
	</form>
	{#if formError}<div class="form-error">{formError}</div>{/if}
	<table>
		<thead>
			<tr><th>Subject</th><th>Role</th><th>Tenant</th><th></th></tr>
		</thead>
		<tbody>
			{#each users as u (u.Subject)}
				<tr>
					<td><strong>{u.Subject}</strong></td>
					<td>
						<span class="badge" style="background:{roleColor(u.Role)}">{u.Role}</span>
					</td>
					<td><code>{u.TenantID}</code></td>
					<td><button class="row-del" onclick={() => removeUser(u.Subject)} title="Remove">✕</button></td>
				</tr>
			{/each}
		</tbody>
	</table>
</ListScaffold>

<style>
	.create-form {
		display: grid;
		grid-template-columns: 2fr 140px 1fr auto;
		gap: 0.5rem;
		margin-bottom: 1rem;
		align-items: center;
	}
	.create-form input,
	.create-form select {
		padding: 0.4rem 0.6rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 4px;
		color: inherit;
		font: inherit;
	}
	.create-form button {
		padding: 0.4rem 1rem;
		background: var(--accent);
		color: white;
		border: none;
		border-radius: 4px;
		cursor: pointer;
		font-weight: 600;
	}
	.create-form button:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.form-error {
		padding: 0.5rem 0.75rem;
		background: rgba(240, 80, 80, 0.15);
		border-left: 3px solid var(--err);
		border-radius: 4px;
		color: var(--err);
		margin-bottom: 1rem;
		font-size: 0.85rem;
	}
	.row-del {
		padding: 0.2rem 0.45rem;
		background: transparent;
		color: var(--muted);
		border: 1px solid var(--border);
		border-radius: 3px;
		cursor: pointer;
		font-size: 0.8rem;
	}
	.row-del:hover {
		background: rgba(240, 80, 80, 0.15);
		color: var(--err);
		border-color: var(--err);
	}
</style>

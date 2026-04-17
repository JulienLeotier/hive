<script lang="ts">
	import { apiGet } from '$lib/api';

	type User = { Subject: string; Role: string; TenantID: string };
	let users = $state<User[]>([]);
	let loading = $state(true);

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

	function roleColor(r: string): string {
		if (r === 'admin') return 'var(--err)';
		if (r === 'operator') return 'var(--accent)';
		return 'var(--text-muted)';
	}
</script>

<main>
	<h1>Users</h1>
	<p class="subtitle">RBAC directory — OIDC subject → role + tenant mapping.</p>

	{#if loading}
		<div class="empty">Loading…</div>
	{:else if users.length === 0}
		<div class="empty">
			No users registered. Add one with <code>hive users add &lt;subject&gt; &lt;role&gt;</code>.
		</div>
	{:else}
		<table>
			<thead>
				<tr><th>Subject</th><th>Role</th><th>Tenant</th></tr>
			</thead>
			<tbody>
				{#each users as u (u.Subject)}
					<tr>
						<td><strong>{u.Subject}</strong></td>
						<td>
							<span class="badge" style="background:{roleColor(u.Role)}">{u.Role}</span>
						</td>
						<td><code>{u.TenantID}</code></td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</main>

<style>
	.subtitle {
		color: var(--text-muted);
		margin-top: 0;
	}
</style>

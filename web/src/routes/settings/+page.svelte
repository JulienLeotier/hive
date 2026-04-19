<script lang="ts">
	import { onMount } from 'svelte';
	import { apiGet, apiPost } from '$lib/api';

	type NotifySettings = {
		slack_enabled: boolean;
		slack_host: string;
		events: string[];
	};

	let settings = $state<NotifySettings | null>(null);
	let loading = $state(true);
	let testing = $state(false);
	let testStatus = $state<'idle' | 'ok' | 'err'>('idle');
	let testMessage = $state('');

	type AdminStats = Record<string, number>;
	let stats = $state<AdminStats | null>(null);
	let sweeping = $state(false);
	let sweepStatus = $state<'idle' | 'ok' | 'err'>('idle');
	let sweepMessage = $state('');

	let busy = $state<'delete-failed' | 'unwedge' | null>(null);
	let busyStatus = $state<'idle' | 'ok' | 'err'>('idle');
	let busyMessage = $state('');

	async function loadStats() {
		try {
			stats = await apiGet<AdminStats>('/api/v1/admin/stats');
		} catch {
			stats = null;
		}
	}

	async function runSweep() {
		sweeping = true;
		sweepStatus = 'idle';
		sweepMessage = '';
		try {
			const r = (await apiPost('/api/v1/admin/sweep', {})) as { rows_deleted: number };
			sweepStatus = 'ok';
			sweepMessage = `${r.rows_deleted} ligne${r.rows_deleted > 1 ? 's' : ''} supprimée${r.rows_deleted > 1 ? 's' : ''}.`;
			await loadStats();
		} catch (e) {
			sweepStatus = 'err';
			sweepMessage = e instanceof Error ? e.message : String(e);
		} finally {
			sweeping = false;
		}
	}

	async function deleteFailed() {
		const { confirmDialog } = await import('$lib/confirm');
		const ok = await confirmDialog({
			title: 'Supprimer tous les projets failed ?',
			message: 'Cette action supprime définitivement tous les projets en status `failed` avec leurs epics/stories/reviews. Irréversible.',
			confirmLabel: 'Supprimer',
			danger: true
		});
		if (!ok) return;
		busy = 'delete-failed';
		busyStatus = 'idle';
		busyMessage = '';
		try {
			const r = (await apiPost('/api/v1/admin/delete-failed', {})) as { deleted: number };
			busyStatus = 'ok';
			busyMessage = `${r.deleted} projet${r.deleted > 1 ? 's' : ''} supprimé${r.deleted > 1 ? 's' : ''}.`;
			await loadStats();
		} catch (e) {
			busyStatus = 'err';
			busyMessage = e instanceof Error ? e.message : String(e);
		} finally {
			busy = null;
		}
	}

	async function unwedgeStories() {
		busy = 'unwedge';
		busyStatus = 'idle';
		busyMessage = '';
		try {
			const r = (await apiPost('/api/v1/admin/unwedge', {})) as { unwedged: number };
			busyStatus = 'ok';
			busyMessage = `${r.unwedged} story${r.unwedged > 1 ? 'ies' : ''} remise en piste.`;
			await loadStats();
		} catch (e) {
			busyStatus = 'err';
			busyMessage = e instanceof Error ? e.message : String(e);
		} finally {
			busy = null;
		}
	}

	async function load() {
		try {
			settings = await apiGet<NotifySettings>('/api/v1/settings/notify');
		} catch {
			/* banner */
		} finally {
			loading = false;
		}
	}

	async function testWebhook() {
		testing = true;
		testStatus = 'idle';
		testMessage = '';
		try {
			await apiPost('/api/v1/settings/notify/test', {});
			testStatus = 'ok';
			testMessage = 'Message envoyé — vérifie ton canal Slack.';
		} catch (e) {
			testStatus = 'err';
			testMessage = e instanceof Error ? e.message : String(e);
		} finally {
			testing = false;
		}
	}

	onMount(() => {
		load();
		loadStats();
	});
</script>

<svelte:head><title>Réglages · Hive</title></svelte:head>

<h1>Réglages</h1>
<p class="sub">
	Configuration read-only. Hive est un outil local : la plupart des réglages vivent dans l'environnement du processus serveur.
</p>

<section class="card">
	<header class="card-header">
		<span class="card-icon">🔔</span>
		<h2 class="card-title">Notifications Slack</h2>
	</header>
	{#if loading}
		<div class="empty">Chargement…</div>
	{:else if settings?.slack_enabled}
		<div class="status-ok">
			<span class="dot"></span>
			Webhook actif — {settings.slack_host}
		</div>
		<p class="muted">
			Hive postera un message Slack sur chacun des événements suivants :
		</p>
		<ul class="events">
			{#each settings.events as evt}
				<li><code>{evt}</code></li>
			{/each}
		</ul>
		<div class="test-row">
			<button type="button" onclick={testWebhook} disabled={testing}>
				{testing ? 'Envoi…' : 'Tester le webhook'}
			</button>
			{#if testStatus === 'ok'}
				<span class="test-msg ok">✓ {testMessage}</span>
			{:else if testStatus === 'err'}
				<span class="test-msg err">✗ {testMessage}</span>
			{/if}
		</div>
	{:else}
		<div class="status-off">
			<span class="dot"></span>
			Webhook non configuré
		</div>
		<p class="muted">
			Pour activer, exporte la variable d'env avant de démarrer le serveur :
		</p>
		<pre class="cmd">export HIVE_SLACK_WEBHOOK="https://hooks.slack.com/services/..."</pre>
		<p class="muted">
			Crée un webhook entrant sur
			<a href="https://api.slack.com/messaging/webhooks" target="_blank" rel="noopener">
				api.slack.com/messaging/webhooks
			</a>, colle l'URL ici puis relance <code>hive serve</code>.
		</p>
	{/if}
</section>

<section class="card">
	<header class="card-header">
		<span class="card-icon">🗃</span>
		<h2 class="card-title">Base de données</h2>
	</header>
	{#if stats}
		<dl class="env">
			{#each Object.entries(stats) as [t, n] (t)}
				<dt><code>{t}</code></dt>
				<dd>{n.toLocaleString('fr-FR')} ligne{n > 1 ? 's' : ''}</dd>
			{/each}
		</dl>
	{/if}
	<p class="muted">
		Nettoyer les events (&gt; 90 jours) et l'audit_log (&gt; 365 jours). Idempotent — rien de sensible n'est touché.
	</p>
	<div class="test-row">
		<button type="button" onclick={runSweep} disabled={sweeping}>
			{sweeping ? 'Nettoyage…' : 'Nettoyer maintenant'}
		</button>
		{#if sweepStatus === 'ok'}
			<span class="test-msg ok">✓ {sweepMessage}</span>
		{:else if sweepStatus === 'err'}
			<span class="test-msg err">✗ {sweepMessage}</span>
		{/if}
	</div>
</section>

<section class="card">
	<header class="card-header">
		<span class="card-icon">🩺</span>
		<h2 class="card-title">Maintenance</h2>
	</header>
	<p class="muted">
		Actions groupées pour nettoyer l'état. À utiliser quand la DB contient plein de projets failed ou des stories coincées dev/review sans devloop actif.
	</p>
	<div class="test-row">
		<button type="button" onclick={deleteFailed} disabled={busy !== null}>
			{busy === 'delete-failed' ? 'Suppression…' : 'Supprimer tous les projets failed'}
		</button>
		<button type="button" onclick={unwedgeStories} disabled={busy !== null}>
			{busy === 'unwedge' ? 'Rewind…' : 'Unwedge stories coincées'}
		</button>
		{#if busyStatus === 'ok'}
			<span class="test-msg ok">✓ {busyMessage}</span>
		{:else if busyStatus === 'err'}
			<span class="test-msg err">✗ {busyMessage}</span>
		{/if}
	</div>
</section>

<section class="card">
	<header class="card-header">
		<span class="card-icon">⚙</span>
		<h2 class="card-title">Autres réglages via env</h2>
	</header>
	<dl class="env">
		<dt><code>HIVE_PORT</code></dt>
		<dd>Port HTTP (défaut : 8080)</dd>
		<dt><code>HIVE_DATA_DIR</code></dt>
		<dd>Répertoire de la base SQLite</dd>
		<dt><code>HIVE_DEV_AGENT</code></dt>
		<dd><code>claude-code</code> (défaut) ou <code>scripted</code> pour tests sans Claude</dd>
		<dt><code>HIVE_DEVLOOP_INTERVAL</code></dt>
		<dd>Cadence du superviseur (défaut : 10s)</dd>
		<dt><code>HIVE_SLACK_WEBHOOK</code></dt>
		<dd>Webhook Slack pour les notifications BMAD</dd>
	</dl>
</section>

<style>
	.sub { color: var(--text-muted); margin: 0 0 1.5rem; font-size: 0.85rem; }
	.card {
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 1.25rem 1.5rem;
		margin-bottom: 1.5rem;
	}
	.card-header {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		margin-bottom: 0.8rem;
		padding-bottom: 0.6rem;
		border-bottom: 1px solid var(--border);
	}
	.card-icon {
		font-size: 1.1rem;
		width: 32px;
		height: 32px;
		border-radius: 8px;
		background: color-mix(in srgb, var(--accent) 15%, transparent);
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
	}
	.card-title {
		margin: 0;
		text-transform: none;
		letter-spacing: 0;
		color: var(--text);
		font-size: 1rem;
		font-weight: 600;
	}
	.status-ok, .status-off {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.35rem 0.75rem;
		border-radius: 999px;
		font-size: 0.8rem;
		margin-bottom: 0.5rem;
	}
	.status-ok {
		background: color-mix(in srgb, var(--ok) 15%, transparent);
		color: var(--ok);
	}
	.status-off {
		background: color-mix(in srgb, var(--text-muted) 15%, transparent);
		color: var(--text-muted);
	}
	.dot {
		width: 8px; height: 8px; border-radius: 50%; background: currentColor;
	}
	.muted { color: var(--text-muted); font-size: 0.85rem; margin: 0.5rem 0; }
	.events {
		list-style: none; padding: 0; margin: 0.5rem 0 0;
		display: flex; flex-wrap: wrap; gap: 0.4rem;
	}
	.events li { margin: 0; }
	.cmd {
		background: var(--bg);
		border: 1px solid var(--border);
		padding: 0.6rem 0.8rem;
		border-radius: 6px;
		font-size: 0.78rem;
		overflow-x: auto;
		margin: 0.5rem 0;
	}
	.env {
		display: grid;
		grid-template-columns: max-content 1fr;
		gap: 0.4rem 1rem;
		margin: 0;
		font-size: 0.85rem;
	}
	.env dt { margin: 0; }
	.env dd { margin: 0; color: var(--text-muted); overflow-wrap: anywhere; }
	@media (max-width: 640px) {
		.env { grid-template-columns: 1fr; gap: 0.1rem; }
		.env dt { margin-top: 0.5rem; }
	}
	a { color: var(--accent); text-decoration: none; }
	a:hover { text-decoration: underline; }
	.test-row {
		display: flex;
		align-items: center;
		gap: 0.8rem;
		margin-top: 0.8rem;
	}
	.test-row button {
		padding: 0.4rem 0.9rem;
		background: var(--accent);
		color: white;
		border: none;
		border-radius: 4px;
		cursor: pointer;
		font-size: 0.85rem;
	}
	.test-row button:disabled { opacity: 0.5; cursor: not-allowed; }
	.test-msg { font-size: 0.8rem; }
	.test-msg.ok { color: var(--ok); }
	.test-msg.err { color: var(--err); }
</style>

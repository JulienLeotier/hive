<script lang="ts">
	import { onMount } from 'svelte';
	import { apiGet } from '$lib/api';

	type NotifySettings = {
		slack_enabled: boolean;
		slack_host: string;
		events: string[];
	};

	let settings = $state<NotifySettings | null>(null);
	let loading = $state(true);

	async function load() {
		try {
			settings = await apiGet<NotifySettings>('/api/v1/settings/notify');
		} catch {
			/* banner */
		} finally {
			loading = false;
		}
	}

	onMount(load);
</script>

<svelte:head><title>Réglages · Hive</title></svelte:head>

<h1>Réglages</h1>
<p class="sub">
	Configuration read-only. Hive est un outil local : la plupart des réglages vivent dans l'environnement du processus serveur.
</p>

<section class="card">
	<h2 class="card-title">Notifications Slack</h2>
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
	<h2 class="card-title">Autres réglages via env</h2>
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
	.card-title {
		margin: 0 0 0.8rem;
		text-transform: none;
		letter-spacing: 0;
		color: var(--text);
		font-size: 1rem;
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
	.env dd { margin: 0; color: var(--text-muted); }
	a { color: var(--accent); text-decoration: none; }
	a:hover { text-decoration: underline; }
</style>

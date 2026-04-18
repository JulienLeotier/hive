<script lang="ts">
	import { apiGet, apiPost, apiDelete } from '$lib/api';
	import { fmtRelative } from '$lib/format';
	import { createReconnectingWS, wsURL } from '$lib/ws';
	import FolderPicker from '$lib/FolderPicker.svelte';
	import ListScaffold from '$lib/ListScaffold.svelte';

	type Project = {
		id: string;
		name: string;
		idea: string;
		prd?: string;
		workdir?: string;
		status: string;
		created_at: string;
		updated_at: string;
		total_cost_usd?: number;
		cost_cap_usd?: number;
		is_existing?: boolean;
		repo_url?: string;
		failure_stage?: string;
		failure_error?: string;
	};

	let projects = $state<Project[]>([]);
	let loading = $state(true);
	let showForm = $state(false);
	let formError = $state('');
	let submitting = $state(false);

	// New project form
	let name = $state('');
	let idea = $state('');
	let workdir = $state('');
	let bmadOutputPath = $state('');
	let repoPath = $state('');
	let costCapUSD = $state('');

	// Intégration GitHub
	type GhStatus = { installed: boolean; authenticated: boolean; login?: string; error?: string };
	type GhRepo = { name_with_owner: string; description?: string; url: string; private: boolean; updated_at?: string };
	let ghStatus = $state<GhStatus | null>(null);
	let ghRepos = $state<GhRepo[]>([]);
	let ghReposLoading = $state(false);
	let ghReposError = $state('');
	let githubMode = $state<'none' | 'clone' | 'create'>('none');
	let cloneRepoTarget = $state('');
	let createRepoName = $state('');
	let repoVisibility = $state<'private' | 'public'>('private');

	async function loadGhStatus() {
		try {
			ghStatus = await apiGet<GhStatus>('/api/v1/gh/status');
		} catch {
			/* silent — UI reste en mode 'none' */
		}
	}

	// Charge la liste des repos à la demande (quand l'utilisateur
	// bascule en mode clone), pas au load initial : `gh repo list`
	// coûte une requête réseau.
	async function loadGhRepos() {
		if (ghReposLoading || ghRepos.length > 0) return;
		ghReposLoading = true;
		ghReposError = '';
		try {
			ghRepos = (await apiGet<GhRepo[]>('/api/v1/gh/repos')) ?? [];
		} catch (e) {
			ghReposError = e instanceof Error ? e.message : String(e);
		} finally {
			ghReposLoading = false;
		}
	}

	// Trigger le load quand le mode passe à clone.
	$effect(() => {
		if (githubMode === 'clone' && ghStatus?.authenticated) {
			loadGhRepos();
		}
	});

	// On considère le projet comme "brownfield" (base de code déjà
	// là) dès qu'on clone un repo OU qu'on a un repo_path local.
	// Le form bascule alors son vocabulaire (idée → feature à
	// ajouter) et l'agent PM côté backend utilisera aussi
	// IterationPipeline au finalize.
	let isBrownfield = $derived(githubMode === 'clone' || repoPath.trim() !== '');

	// Login GitHub par PAT depuis l'UI.
	let ghToken = $state('');
	let ghLoggingIn = $state(false);
	let ghLoginError = $state('');

	async function ghLogin() {
		if (!ghToken.trim()) return;
		ghLoggingIn = true;
		ghLoginError = '';
		try {
			ghStatus = (await apiPost('/api/v1/gh/login', { token: ghToken })) as GhStatus;
			ghToken = '';
		} catch (e) {
			ghLoginError = e instanceof Error ? e.message : String(e);
		} finally {
			ghLoggingIn = false;
		}
	}

	// OAuth device flow (alternative au PAT) : l'user clique, on lance
	// /gh/device/start pour obtenir un user_code, on ouvre l'URL de
	// vérification dans un onglet, puis on poll /gh/device/poll jusqu'à
	// obtenir le token (ou expiration / refus).
	type DeviceStart = {
		user_code: string;
		verification_uri: string;
		device_code: string;
		interval: number;
		expires_in: number;
	};
	let device = $state<DeviceStart | null>(null);
	let deviceStatus = $state<'idle' | 'waiting' | 'ok' | 'error'>('idle');
	let deviceError = $state('');
	let devicePoll: ReturnType<typeof setTimeout> | null = null;

	async function startDevice() {
		deviceError = '';
		deviceStatus = 'waiting';
		try {
			device = (await apiPost('/api/v1/gh/device/start', {})) as DeviceStart;
			window.open(device.verification_uri, '_blank', 'noopener');
			scheduleDevicePoll();
		} catch (e) {
			deviceStatus = 'error';
			deviceError = e instanceof Error ? e.message : String(e);
		}
	}

	function scheduleDevicePoll() {
		if (!device) return;
		devicePoll = setTimeout(pollDevice, Math.max(device.interval, 5) * 1000);
	}

	async function pollDevice() {
		if (!device) return;
		try {
			const res = await fetch('/api/v1/gh/device/poll', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ device_code: device.device_code })
			});
			if (res.status === 202) {
				scheduleDevicePoll();
				return;
			}
			const body = await res.json();
			if (!res.ok) {
				deviceStatus = 'error';
				deviceError = body?.error?.message ?? 'échec device flow';
				device = null;
				return;
			}
			ghStatus = body as GhStatus;
			deviceStatus = 'ok';
			device = null;
		} catch (e) {
			deviceStatus = 'error';
			deviceError = e instanceof Error ? e.message : String(e);
		}
	}

	function cancelDevice() {
		if (devicePoll) clearTimeout(devicePoll);
		devicePoll = null;
		device = null;
		deviceStatus = 'idle';
	}

	async function ghLogout() {
		try {
			ghStatus = (await apiPost('/api/v1/gh/logout', {})) as GhStatus;
		} catch {
			/* on rafraîchit quand même pour voir l'état réel */
			await loadGhStatus();
		}
	}

	async function load() {
		try {
			projects = (await apiGet<Project[]>('/api/v1/projects')) ?? [];
		} catch {
			/* banner */
		} finally {
			loading = false;
		}
	}

	// Any story.* or project.* event means a project row on this page
	// probably just flipped status — refresh the list rather than the
	// whole per-row payload (cheap, single query).
	function shouldRefresh(type: string): boolean {
		return (
			type.startsWith('story.') ||
			type.startsWith('project.') ||
			type === 'intake.finalized'
		);
	}

	$effect(() => {
		load();
		loadGhStatus();
		const i = setInterval(load, 15000);
		const ws = createReconnectingWS({
			url: wsURL('/ws'),
			onmessage: (msg) => {
				try {
					const evt = JSON.parse(msg.data) as { type?: string };
					if (!evt.type || !shouldRefresh(evt.type)) return;
					load();
				} catch {
					/* ignore non-JSON frames */
				}
			}
		});
		return () => {
			clearInterval(i);
			ws.close();
		};
	});

	async function createProject(ev: Event) {
		ev.preventDefault();
		formError = '';
		submitting = true;
		try {
			const payload: Record<string, unknown> = {
				name,
				idea,
				workdir,
				bmad_output_path: bmadOutputPath,
				repo_path: repoPath
			};
			if (githubMode === 'clone' && cloneRepoTarget.trim()) {
				payload.clone_repo = cloneRepoTarget.trim();
			} else if (githubMode === 'create' && createRepoName.trim()) {
				payload.create_repo = createRepoName.trim();
				payload.repo_visibility = repoVisibility;
			}
			const cap = parseFloat(costCapUSD);
			if (!Number.isNaN(cap) && cap > 0) {
				payload.cost_cap_usd = cap;
			}
			const p = (await apiPost('/api/v1/projects', payload)) as Project;
			window.location.href = `/projects/${encodeURIComponent(p.id)}`;
		} catch (e) {
			formError = e instanceof Error ? e.message : String(e);
		} finally {
			submitting = false;
		}
	}

	async function removeProject(id: string, label: string) {
		if (!confirm(`Supprimer le projet « ${label} » ? Ses epics, stories et historique de revue seront aussi effacés.`))
			return;
		const purgeWorkdir = confirm(
			`Veux-tu AUSSI effacer le répertoire de travail sur disque ? ` +
				`(annuler = garder les fichiers, OK = rm -rf)`
		);
		try {
			const qs = purgeWorkdir ? '?purge_workdir=true' : '';
			await apiDelete(`/api/v1/projects/${encodeURIComponent(id)}${qs}`);
			await load();
		} catch (e) {
			formError = e instanceof Error ? e.message : String(e);
		}
	}

	function statusColor(s: string): string {
		const map: Record<string, string> = {
			draft: 'var(--text-muted)',
			planning: 'var(--accent)',
			building: 'var(--warn)',
			review: 'var(--warn)',
			shipped: 'var(--ok)',
			failed: 'var(--err)'
		};
		return map[s] ?? 'var(--text-muted)';
	}
</script>

<ListScaffold
	title="Projets"
	subtitle="Chaque projet est une construction autonome. Décris ce que tu veux ; les agents BMAD font le reste — le PM rédige un PRD, l'Architecte décompose, le Dev code, le Relecteur valide, jusqu'à ce que tous les critères d'acceptation passent."
	{loading}
	isEmpty={projects.length === 0 && !showForm}
	emptyText="Aucun projet pour l'instant. Décris ce que tu veux construire."
>
	{#snippet controls()}
		<div class="toolbar">
			<button class="new-btn" class:open={showForm} onclick={() => (showForm = !showForm)}>
				<span class="new-icon">{showForm ? '✕' : '+'}</span>
				{showForm ? 'Fermer' : 'Nouveau projet'}
			</button>
		</div>
	{/snippet}

	{#if showForm}
		<form class="create-form" class:brownfield={isBrownfield} onsubmit={createProject}>
			<header class="form-head">
				<h2 class="form-title">
					{#if isBrownfield}
						🏗 Nouveau projet (brownfield)
					{:else}
						✨ Nouveau projet
					{/if}
				</h2>
				{#if isBrownfield}
					<p class="form-sub">
						Tu pars d'un code existant. Hive lancera <code>/bmad-document-project</code>
						puis <code>/bmad-edit-prd</code> au lieu du pipeline from-scratch.
					</p>
				{:else}
					<p class="form-sub">
						Décris ton idée. BMAD produira le PRD, l'architecture, les stories et le code.
					</p>
				{/if}
			</header>

			<fieldset class="section">
				<legend><span class="sn">1</span> Idée</legend>
				<label class="field">
					<span class="label">
						{isBrownfield
							? "Qu'est-ce que tu veux ajouter ou améliorer ?"
							: "Qu'est-ce que tu veux construire ?"}
					</span>
					<textarea
						rows="3"
						placeholder={isBrownfield
							? "Ex. ajouter l'export PDF avec mise en page personnalisée."
							: "Ex. une app qui aide les romanciers à écrire, éditer et recevoir du feedback IA."}
						bind:value={idea}
						required></textarea>
					<small>
						{isBrownfield
							? "Décris la feature ou correction. L'agent PM te posera les questions de suivi."
							: "Une phrase claire suffit. L'agent PM cadrera à l'étape suivante."}
					</small>
				</label>
				<label class="field">
					<span class="label">Nom court <span class="opt">optionnel</span></span>
					<input type="text" placeholder="auto-généré si vide" bind:value={name} />
				</label>
			</fieldset>

			<fieldset class="section">
				<legend><span class="sn">2</span> Emplacement</legend>
				<label class="field">
					<span class="label">Répertoire de travail</span>
					<FolderPicker bind:value={workdir}
						placeholder="/Users/moi/projects/mon-app"
						label="Choisir le répertoire de travail" />
					<small>Le Dev commite ici. <strong>Évite</strong> un dossier personnel (Documents, Desktop, Downloads) — Hive refusera.</small>
				</label>
				<label class="field">
					<span class="label">Dossier BMAD existant <span class="opt">optionnel</span></span>
					<FolderPicker bind:value={bmadOutputPath}
						placeholder="/Users/moi/bmad-output/mon-app"
						label="Choisir le dossier BMAD existant" />
					<small>Si tu as déjà un PRD/epics BMAD ailleurs, on les réutilise.</small>
				</label>
				{#if githubMode !== 'clone'}
					<label class="field">
						<span class="label">Repo local existant <span class="opt">optionnel</span></span>
						<FolderPicker bind:value={repoPath}
							placeholder="/Users/moi/projects/mon-repo-existant"
							label="Choisir un repo local existant" />
						<small>Ajoute BMAD à une base de code locale (sans passer par GitHub).</small>
					</label>
				{/if}
			</fieldset>

			<fieldset class="section gh">
				<legend>
					<span class="sn">3</span> GitHub
					{#if ghStatus?.authenticated}
						<span class="gh-pill ok" title="Authentifié via gh">✓ {ghStatus.login}</span>
						<button type="button" class="link" onclick={ghLogout}>Se déconnecter</button>
					{:else if ghStatus?.installed}
						<span class="gh-pill warn">⚠ non connecté</span>
					{:else if ghStatus}
						<span class="gh-pill warn" title={ghStatus.error}>gh non installé</span>
					{/if}
				</legend>

				{#if ghStatus?.installed && !ghStatus?.authenticated}
					<div class="gh-login">
						<p class="gh-hint">
							Colle un personal access token GitHub pour connecter Hive.
							<a href="https://github.com/settings/tokens/new?scopes=repo,workflow,read:org&description=Hive%20BMAD"
								target="_blank" rel="noopener">
								Créer un token
							</a>
							(scopes requis : <code>repo</code>, <code>workflow</code>, <code>read:org</code>).
						</p>
						<div class="gh-login-row">
							<input type="password"
								placeholder="ghp_..."
								bind:value={ghToken}
								autocomplete="off" />
							<button type="button"
								onclick={ghLogin}
								disabled={ghLoggingIn || !ghToken.trim()}>
								{ghLoggingIn ? 'Connexion…' : 'Se connecter'}
							</button>
						</div>
						{#if ghLoginError}
							<div class="err">{ghLoginError}</div>
						{/if}
						<div class="gh-or">— ou —</div>
						{#if !device}
							<button type="button"
								class="gh-device-btn"
								onclick={startDevice}
								disabled={deviceStatus === 'waiting'}>
								Se connecter via navigateur (OAuth)
							</button>
						{:else}
							<div class="gh-device-modal">
								<p>
									Ouvre <a href={device.verification_uri} target="_blank" rel="noopener">{device.verification_uri}</a>
									et saisis ce code :
								</p>
								<code class="gh-device-code">{device.user_code}</code>
								<p class="gh-hint">Hive poll GitHub toutes les {device.interval}s — reviens ici quand c'est validé.</p>
								<button type="button" class="ghost" onclick={cancelDevice}>Annuler</button>
							</div>
						{/if}
						{#if deviceError}
							<div class="err">{deviceError}</div>
						{/if}
					</div>
				{/if}

				<label class="radio">
					<input type="radio" name="ghmode" value="none" bind:group={githubMode} />
					Aucune — projet local uniquement
				</label>
				<label class="radio" class:disabled={!ghStatus?.authenticated}>
					<input type="radio" name="ghmode" value="clone"
						bind:group={githubMode}
						disabled={!ghStatus?.authenticated} />
					Cloner un repo GitHub existant
				</label>
				{#if githubMode === 'clone'}
					<div class="gh-clone">
						<input type="text"
							class="gh-input"
							list="gh-repo-list"
							placeholder={ghReposLoading
								? 'Chargement des repos…'
								: 'Choisis un repo ou tape user/repo'}
							bind:value={cloneRepoTarget}
							required />
						<datalist id="gh-repo-list">
							{#each ghRepos as r (r.name_with_owner)}
								<option value={r.name_with_owner}>
									{r.description ?? ''}{r.private ? ' (privé)' : ''}
								</option>
							{/each}
						</datalist>
						{#if ghReposError}
							<small class="gh-hint" style="color:var(--err)">{ghReposError}</small>
						{:else if ghRepos.length > 0}
							<small class="gh-hint">{ghRepos.length} repos disponibles — tape pour filtrer</small>
						{/if}
					</div>
				{/if}

				<label class="radio" class:disabled={!ghStatus?.authenticated}>
					<input type="radio" name="ghmode" value="create"
						bind:group={githubMode}
						disabled={!ghStatus?.authenticated} />
					Créer un nouveau repo GitHub
				</label>
				{#if githubMode === 'create'}
					<div class="gh-create">
						<input type="text"
							placeholder="nom-du-repo"
							bind:value={createRepoName}
							required />
						<select bind:value={repoVisibility}>
							<option value="private">Privé</option>
							<option value="public">Public</option>
						</select>
					</div>
				{/if}

				{#if githubMode !== 'none' && !workdir.trim()}
					<small class="gh-hint">
						Le workdir ci-dessus est requis : c'est là que Hive va cloner ou initialiser le repo.
					</small>
				{/if}
			</fieldset>

			<fieldset class="section">
				<legend><span class="sn">4</span> Budget <span class="opt">optionnel</span></legend>
				<label class="field">
					<span class="label">Plafond de coût Claude (USD)</span>
					<input type="number"
						min="0"
						step="0.5"
						placeholder="ex. 10"
						bind:value={costCapUSD} />
					<small>Hive annule le run BMAD si le cumul dépasse ce montant. Vide = illimité.</small>
				</label>
			</fieldset>

			<div class="form-submit">
				<button type="submit" class="btn-submit" disabled={submitting || !idea.trim()}>
					{submitting
						? '⏳ Création…'
						: isBrownfield
							? '🏗 Créer le projet (brownfield)'
							: '✨ Créer le projet'}
				</button>
				{#if formError}<div class="form-error">{formError}</div>{/if}
			</div>
		</form>
	{/if}

	<ul class="pj-list">
		{#each projects as p (p.id)}
			<li class="pj-card" class:shipped={p.status === 'shipped'}
				class:failed={p.status === 'failed'}
				class:running={p.status === 'planning' || p.status === 'building' || p.status === 'review'}>
				<a class="pj-link" href="/projects/{p.id}">
					<span class="pj-dot" style="background:{statusColor(p.status)}"
						class:pulsing={p.status === 'planning' || p.status === 'building'}></span>
					<div class="pj-body">
						<div class="pj-top">
							<strong class="pj-name">{p.name}</strong>
							<span class="badge pj-status" style="background:{statusColor(p.status)}">{p.status}</span>
						</div>
						<p class="pj-idea">{p.idea}</p>
						<div class="pj-foot">
							<span class="pj-time">🕒 {fmtRelative(p.updated_at)}</span>
							{#if (p.total_cost_usd ?? 0) > 0}
								<span class="pj-cost" title="Cumul tokens Claude">
									💰 ${(p.total_cost_usd ?? 0).toFixed(2)}
								</span>
							{/if}
							{#if p.is_existing}
								<span class="pj-tag brownfield" title="Projet basé sur un repo existant">🏗 brownfield</span>
							{/if}
							{#if p.failure_stage}
								<span class="pj-tag err" title={p.failure_error ?? ''}>✕ {p.failure_stage}</span>
							{/if}
						</div>
					</div>
					<span class="pj-chevron" aria-hidden="true">›</span>
				</a>
				<button class="pj-del"
					onclick={() => removeProject(p.id, p.name)}
					title="Supprimer"
					aria-label="Supprimer {p.name}">✕</button>
			</li>
		{/each}
		{#if projects.length === 0 && !loading}
			<li class="list-empty">
				<span class="empty-icon">📦</span>
				<div>
					<strong>Aucun projet pour l'instant.</strong>
					<span>Clique sur « Nouveau projet » ci-dessus pour démarrer.</span>
				</div>
			</li>
		{/if}
	</ul>
</ListScaffold>

<style>
	.toolbar {
		margin: 1rem 0 1.25rem;
	}
	.new-btn {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.6rem 1.1rem;
		background: var(--accent);
		color: white;
		border: none;
		border-radius: 8px;
		cursor: pointer;
		font-weight: 600;
		font-size: 0.9rem;
		transition: background 0.1s, transform 0.05s;
	}
	.new-btn:hover { background: color-mix(in srgb, var(--accent) 88%, black); }
	.new-btn.open { background: var(--bg-hover); color: var(--text); }
	.new-btn.open:hover { background: var(--border); }
	.new-icon {
		display: inline-flex;
		width: 20px;
		height: 20px;
		align-items: center;
		justify-content: center;
		font-size: 1rem;
		line-height: 1;
	}
	/* ===== Create form ===== */
	.create-form {
		display: flex;
		flex-direction: column;
		gap: 1rem;
		padding: 1.5rem;
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 12px;
		margin-bottom: 1.5rem;
		transition: border-color 200ms ease, box-shadow 200ms ease;
	}
	.create-form.brownfield {
		border-color: color-mix(in srgb, var(--accent) 50%, var(--border));
		box-shadow: 0 0 0 1px color-mix(in srgb, var(--accent) 20%, transparent);
	}
	.form-head {
		padding-bottom: 0.85rem;
		border-bottom: 1px solid var(--border);
	}
	.form-title {
		margin: 0 0 0.3rem;
		font-size: 1.15rem;
		font-weight: 700;
		color: var(--text);
		text-transform: none;
		letter-spacing: 0;
	}
	.form-sub {
		margin: 0;
		font-size: 0.85rem;
		color: var(--text-muted);
		line-height: 1.55;
	}
	.form-sub code {
		background: var(--bg-hover);
		color: var(--accent);
		padding: 1px 6px;
		border-radius: 3px;
		font-size: 0.78rem;
	}
	.section {
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 1rem 1.1rem 0.9rem;
		background: var(--bg-alt);
		display: flex;
		flex-direction: column;
		gap: 0.8rem;
		margin: 0;
	}
	.section legend {
		padding: 0 0.5rem;
		font-size: 0.78rem;
		color: var(--text);
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
	}
	.sn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 22px;
		height: 22px;
		border-radius: 50%;
		background: color-mix(in srgb, var(--accent) 15%, transparent);
		color: var(--accent);
		font-size: 0.75rem;
		font-weight: 700;
		text-transform: none;
		letter-spacing: 0;
	}
	.field {
		display: flex;
		flex-direction: column;
		gap: 0.35rem;
	}
	.field .label {
		font-size: 0.82rem;
		font-weight: 600;
		color: var(--text);
		display: flex;
		align-items: center;
		gap: 0.4rem;
	}
	.opt {
		font-size: 0.66rem;
		padding: 1px 8px;
		border-radius: 999px;
		background: var(--bg-hover);
		color: var(--text-muted);
		font-weight: 500;
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}
	.section input:not([type='radio']),
	.section textarea,
	.section select {
		padding: 0.55rem 0.75rem;
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 6px;
		color: inherit;
		font: inherit;
		font-size: 0.9rem;
		transition: border-color 0.1s;
	}
	.section input:focus,
	.section textarea:focus,
	.section select:focus {
		outline: none;
		border-color: var(--accent);
	}
	.section textarea {
		resize: vertical;
		font-family: inherit;
		min-height: 80px;
		line-height: 1.55;
	}
	.section small {
		font-size: 0.76rem;
		color: var(--text-muted);
		line-height: 1.45;
	}
	.section small strong { color: var(--warn); font-weight: 600; }

	.form-submit {
		display: flex;
		flex-direction: column;
		gap: 0.6rem;
	}
	.btn-submit {
		padding: 0.85rem 1.5rem;
		background: var(--accent);
		color: white;
		border: none;
		border-radius: 8px;
		cursor: pointer;
		font-weight: 700;
		font-size: 0.95rem;
		transition: background 0.1s, transform 0.05s;
	}
	.btn-submit:hover:not(:disabled) {
		background: color-mix(in srgb, var(--accent) 88%, black);
	}
	.btn-submit:active:not(:disabled) { transform: translateY(1px); }
	.btn-submit:disabled { opacity: 0.5; cursor: not-allowed; }

	.gh-pill {
		display: inline-block;
		margin-left: 0.4rem;
		padding: 0.05rem 0.5rem;
		border-radius: 999px;
		font-size: 0.7rem;
		font-weight: 500;
		text-transform: none;
		letter-spacing: 0;
	}
	.gh-pill.ok { background: color-mix(in srgb, var(--ok) 25%, transparent); color: var(--ok); }
	.gh-pill.warn { background: color-mix(in srgb, var(--warn) 25%, transparent); color: var(--warn); }
	.radio {
		flex-direction: row !important;
		align-items: center;
		gap: 0.4rem;
		font-size: 0.85rem;
		color: var(--text);
	}
	.radio.disabled { opacity: 0.5; }
	.gh-input, .gh-create input, .gh-create select {
		padding: 0.45rem 0.65rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 4px;
		color: inherit;
		font: inherit;
	}
	.gh-clone { display: flex; flex-direction: column; gap: 0.3rem; }
	.gh-clone .gh-input { font-family: ui-monospace, monospace; font-size: 0.85rem; }
	.gh-create { display: flex; gap: 0.5rem; }
	.gh-create input { flex: 1; }
	.gh-hint { color: var(--muted); font-size: 0.78rem; line-height: 1.4; }
	.gh-hint a { color: var(--accent); }
	.gh-login {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
		padding: 0.6rem;
		background: var(--bg-alt);
		border: 1px dashed var(--border);
		border-radius: 4px;
	}
	.gh-login-row {
		display: flex;
		gap: 0.5rem;
	}
	.gh-login-row input { flex: 1; font-family: ui-monospace, monospace; font-size: 0.8rem; padding: 0.4rem 0.6rem; border: 1px solid var(--border); border-radius: 4px; background: var(--bg); color: inherit; }
	.gh-login-row button { padding: 0.4rem 0.9rem; background: var(--accent); color: white; border: none; border-radius: 4px; cursor: pointer; font-weight: 600; }
	.gh-login-row button:disabled { opacity: 0.5; cursor: not-allowed; }
	.gh-or {
		text-align: center;
		color: var(--muted);
		font-size: 0.75rem;
		margin: 0.5rem 0;
	}
	.gh-device-btn {
		width: 100%;
		padding: 0.5rem 0.8rem;
		background: var(--bg);
		color: var(--fg);
		border: 1px solid var(--border);
		border-radius: 4px;
		cursor: pointer;
		font-weight: 500;
	}
	.gh-device-btn:hover { border-color: var(--accent); }
	.gh-device-modal {
		padding: 0.7rem;
		background: color-mix(in srgb, var(--accent) 10%, transparent);
		border: 1px dashed var(--accent);
		border-radius: 4px;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
		text-align: center;
	}
	.gh-device-code {
		font-family: ui-monospace, monospace;
		font-size: 1.3rem;
		font-weight: 700;
		letter-spacing: 0.2rem;
		padding: 0.4rem 0.8rem;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 4px;
		display: inline-block;
	}
	.link {
		background: none;
		border: none;
		color: var(--muted);
		font-size: 0.7rem;
		cursor: pointer;
		text-decoration: underline;
		padding: 0 0.25rem;
	}
	.link:hover { color: var(--err); }
	.err {
		padding: 0.4rem 0.6rem;
		background: rgba(240, 80, 80, 0.15);
		border-left: 3px solid var(--err);
		border-radius: 3px;
		color: var(--err);
		font-size: 0.78rem;
	}
	.form-error {
		padding: 0.5rem 0.75rem;
		background: rgba(240, 80, 80, 0.15);
		border-left: 3px solid var(--err);
		border-radius: 4px;
		color: var(--err);
		font-size: 0.85rem;
	}
	/* ===== Liste de projets : cards modernes ===== */
	.pj-list {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 0.55rem;
	}
	.pj-card {
		position: relative;
		display: flex;
		align-items: stretch;
		background: var(--bg-panel);
		border: 1px solid var(--border);
		border-radius: 10px;
		transition: border-color 0.15s, transform 0.1s, box-shadow 0.15s;
		overflow: hidden;
	}
	.pj-card:hover {
		border-color: var(--accent);
		transform: translateY(-1px);
		box-shadow: 0 4px 12px color-mix(in srgb, var(--accent) 15%, transparent);
	}
	.pj-card.failed { border-left: 3px solid var(--err); }
	.pj-card.shipped { border-left: 3px solid var(--ok); }
	.pj-card.running { border-left: 3px solid var(--warn); }

	.pj-link {
		display: flex;
		align-items: center;
		gap: 0.9rem;
		flex: 1;
		min-width: 0;
		padding: 0.9rem 1rem;
		color: inherit;
		text-decoration: none;
	}
	.pj-dot {
		width: 12px;
		height: 12px;
		border-radius: 50%;
		flex-shrink: 0;
	}
	.pj-dot.pulsing {
		box-shadow: 0 0 0 0 currentColor;
		animation: dot-pulse 2s ease-in-out infinite;
	}
	@keyframes dot-pulse {
		0% { box-shadow: 0 0 0 0 color-mix(in srgb, currentColor 60%, transparent); }
		70% { box-shadow: 0 0 0 8px transparent; }
		100% { box-shadow: 0 0 0 0 transparent; }
	}
	.pj-body {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}
	.pj-top {
		display: flex;
		align-items: center;
		gap: 0.6rem;
		min-width: 0;
	}
	.pj-name {
		font-size: 0.98rem;
		font-weight: 600;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		min-width: 0;
		flex: 1;
	}
	.pj-link:hover .pj-name { color: var(--accent); }
	.pj-status {
		flex-shrink: 0;
		text-transform: lowercase;
		letter-spacing: 0.03em;
		font-size: 0.68rem;
		padding: 0.12rem 0.6rem;
		border-radius: 999px;
	}
	.pj-idea {
		margin: 0;
		font-size: 0.85rem;
		color: var(--text-muted);
		line-height: 1.5;
		overflow: hidden;
		text-overflow: ellipsis;
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		white-space: normal;
	}
	.pj-foot {
		display: flex;
		gap: 0.5rem;
		flex-wrap: wrap;
		font-size: 0.72rem;
		color: var(--text-muted);
		margin-top: 0.15rem;
	}
	.pj-time { font-variant-numeric: tabular-nums; }
	.pj-cost {
		font-family: ui-monospace, monospace;
		color: var(--warn);
		font-weight: 500;
	}
	.pj-tag {
		padding: 0.1rem 0.55rem;
		background: var(--bg-hover);
		border-radius: 999px;
		font-size: 0.68rem;
		font-weight: 500;
	}
	.pj-tag.brownfield {
		background: color-mix(in srgb, var(--accent) 14%, transparent);
		color: var(--accent);
	}
	.pj-tag.err {
		background: color-mix(in srgb, var(--err) 14%, transparent);
		color: var(--err);
	}
	.pj-chevron {
		font-size: 1.3rem;
		color: var(--text-muted);
		opacity: 0.4;
		transition: opacity 0.1s, transform 0.1s;
		flex-shrink: 0;
		line-height: 1;
	}
	.pj-link:hover .pj-chevron {
		opacity: 1;
		color: var(--accent);
		transform: translateX(2px);
	}
	.pj-del {
		padding: 0 0.85rem;
		background: transparent;
		color: var(--text-muted);
		border: none;
		border-left: 1px solid var(--border);
		cursor: pointer;
		font-size: 1rem;
		min-width: 44px;
		transition: background 0.1s, color 0.1s;
	}
	.pj-del:hover {
		background: color-mix(in srgb, var(--err) 12%, transparent);
		color: var(--err);
	}

	.list-empty {
		display: flex;
		align-items: center;
		gap: 1rem;
		padding: 2rem 1.5rem;
		background: var(--bg-panel);
		border: 1px dashed var(--border);
		border-radius: 10px;
		text-align: left;
	}
	.list-empty > div {
		display: flex;
		flex-direction: column;
		gap: 0.15rem;
	}
	.list-empty strong { font-size: 0.95rem; color: var(--text); }
	.list-empty span { font-size: 0.85rem; color: var(--text-muted); }
	.empty-icon {
		font-size: 2rem;
		opacity: 0.5;
	}

	@media (max-width: 767px) {
		.pj-link { padding: 0.75rem 0.85rem; gap: 0.7rem; }
		.pj-top { flex-wrap: wrap; }
		.pj-name { flex: 1 1 100%; }
		.pj-chevron { display: none; }
		.pj-del { border-left: none; border-top: 1px solid var(--border); padding: 0.5rem 0.85rem; min-width: auto; align-self: stretch; }
		.pj-card { flex-direction: column; }
	}
</style>

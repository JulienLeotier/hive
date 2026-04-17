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
		try {
			await apiDelete(`/api/v1/projects/${encodeURIComponent(id)}`);
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
			<button class="btn primary" onclick={() => (showForm = !showForm)}>
				{showForm ? 'Fermer' : '+ Nouveau projet'}
			</button>
		</div>
	{/snippet}

	{#if showForm}
		<form class="create-form" class:brownfield={isBrownfield} onsubmit={createProject}>
			{#if isBrownfield}
				<div class="brownfield-banner">
					🏗 <strong>Mode brownfield</strong> — tu pars d'un code existant.
					Hive lancera <code>/bmad-document-project</code> puis
					<code>/bmad-edit-prd</code> au lieu du pipeline from-scratch.
				</div>
			{/if}
			<label>
				{isBrownfield
					? "Qu'est-ce que tu veux ajouter ou améliorer ?"
					: "Qu'est-ce que tu veux construire ?"}
				<textarea
					rows="3"
					placeholder={isBrownfield
						? 'Ex. ajouter l\'export PDF avec mise en page personnalisée ; ou : remplacer l\'API REST par du GraphQL.'
						: 'Ex. une app qui aide les romanciers à écrire, éditer et recevoir du feedback IA sur leurs textes.'}
					bind:value={idea}
					required
				></textarea>
				<small>
					{isBrownfield
						? "Décris la feature ou l'amélioration à faire sur ce code. L'agent PM te posera des questions de suivi."
						: "Une phrase claire. L'agent PM te posera des questions de suivi à l'étape suivante."}
				</small>
			</label>
			<label>
				Nom court
				<input type="text" placeholder="auto-généré si vide" bind:value={name} />
			</label>
			<label>
				Répertoire de travail
				<FolderPicker bind:value={workdir}
					placeholder="/Users/moi/projects/writers-app (optionnel)"
					label="Choisir le répertoire de travail" />
				<small>C'est là que le Dev commitera le code. Peut être défini plus tard.</small>
			</label>
			<label>
				Dossier BMAD existant <span class="hint-pill">optionnel</span>
				<FolderPicker bind:value={bmadOutputPath}
					placeholder="/Users/moi/bmad-output/writers-app"
					label="Choisir le dossier BMAD existant" />
				<small>Si tu as déjà lancé BMAD ailleurs (PRD, epics, stories), pointe vers ce dossier et l'Architecte réutilisera les artefacts existants.</small>
			</label>
			{#if githubMode !== 'clone'}
				<label>
					Repo existant (chemin local) <span class="hint-pill">optionnel</span>
					<FolderPicker bind:value={repoPath}
						placeholder="/Users/moi/projects/mon-app-existante"
						label="Choisir un repo local existant" />
					<small>Ajoute BMAD à une base de code existante (sans la cloner depuis GitHub). Les agents Dev travaillent dans ce repo au lieu de scaffolder à partir de zéro.</small>
				</label>
			{/if}

			<fieldset class="gh">
				<legend>
					Intégration GitHub
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

			<fieldset class="block">
				<legend>Budget (optionnel)</legend>
				<label>
					<span>Plafond de coût Claude (USD)</span>
					<input type="number"
						min="0"
						step="0.5"
						placeholder="ex. 10"
						bind:value={costCapUSD} />
					<small>Hive annule le run BMAD si le cumul dépasse ce montant. Laisse vide pour aucun plafond.</small>
				</label>
			</fieldset>

			<button type="submit" disabled={submitting || !idea.trim()}>
				{submitting
					? 'Création…'
					: isBrownfield
						? 'Créer le projet (brownfield)'
						: 'Créer le projet'}
			</button>
			{#if formError}<div class="form-error">{formError}</div>{/if}
		</form>
	{/if}

	<table>
		<thead>
			<tr>
				<th>Projet</th><th>Statut</th><th>Mis à jour</th><th></th>
			</tr>
		</thead>
		<tbody>
			{#each projects as p (p.id)}
				<tr>
					<td>
						<a class="pjrow" href="/projects/{p.id}">
							<strong>{p.name}</strong>
							<span class="muted">{p.idea}</span>
						</a>
					</td>
					<td><span class="badge" style="background:{statusColor(p.status)}">{p.status}</span></td>
					<td>{fmtRelative(p.updated_at)}</td>
					<td>
						<button class="row-del" onclick={() => removeProject(p.id, p.name)} title="Supprimer">✕</button>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</ListScaffold>

<style>
	.toolbar {
		margin: 1rem 0;
	}
	.btn.primary {
		padding: 0.5rem 1rem;
		background: var(--accent);
		color: white;
		border: none;
		border-radius: 6px;
		cursor: pointer;
		font-weight: 600;
	}
	.create-form {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		padding: 1rem;
		background: var(--bg-alt);
		border: 1px solid var(--border);
		border-radius: 6px;
		margin-bottom: 1rem;
		transition: border-color 200ms ease;
	}
	.create-form.brownfield {
		border-color: var(--accent);
		box-shadow: 0 0 0 1px color-mix(in srgb, var(--accent) 30%, transparent);
	}
	.brownfield-banner {
		padding: 0.6rem 0.85rem;
		background: color-mix(in srgb, var(--accent) 14%, var(--bg));
		border-left: 3px solid var(--accent);
		border-radius: 0 4px 4px 0;
		font-size: 0.85rem;
		color: var(--text);
	}
	.brownfield-banner code {
		font-family: ui-monospace, monospace;
		font-size: 0.78rem;
		background: var(--bg-alt);
		padding: 0.05rem 0.3rem;
		border-radius: 3px;
	}
	.create-form label {
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
		font-size: 0.85rem;
		color: var(--muted);
	}
	.create-form input,
	.create-form textarea {
		padding: 0.5rem 0.7rem;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 4px;
		color: inherit;
		font: inherit;
	}
	.create-form textarea {
		resize: vertical;
		font-family: inherit;
	}
	.create-form button {
		align-self: flex-start;
		padding: 0.5rem 1rem;
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
	.create-form small {
		font-size: 0.75rem;
		color: var(--muted);
	}
	.hint-pill {
		display: inline-block;
		margin-left: 0.4rem;
		padding: 0 0.4rem;
		font-size: 0.65rem;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 999px;
		color: var(--muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}
	.gh {
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 0.75rem 1rem;
		background: var(--bg);
		display: flex;
		flex-direction: column;
		gap: 0.4rem;
	}
	.gh legend {
		font-size: 0.78rem;
		color: var(--muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		padding: 0 0.25rem;
	}
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
	.pjrow {
		display: flex;
		flex-direction: column;
		gap: 0.15rem;
		color: inherit;
		text-decoration: none;
	}
	.pjrow:hover strong {
		color: var(--accent);
	}
	.muted {
		color: var(--muted);
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

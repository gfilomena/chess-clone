<script lang="ts">
	import { onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { user, authLoading } from '$lib/stores/auth';
	import {
		onlineUsers,
		fetchOnlineUsers,
		sendInvite
	} from '$lib/stores/invitations';
	import { API_URL as API } from '$lib/config';

	// ── Auth guard ────────────────────────────────────────────────────────────────
	$effect(() => {
		if (!$authLoading && !$user) goto('/login');
	});

	// ── Matchmaking ───────────────────────────────────────────────────────────────
	let mm: 'idle' | 'searching' | 'found' | 'error' = $state('idle');
	let waitSeconds = $state(0);
	let errorMsg = $state('');
	let eventSource: EventSource | null = null;
	let waitTimer: ReturnType<typeof setInterval> | null = null;

	// ── Friend invite ─────────────────────────────────────────────────────────────
	let fi: 'idle' | 'pending' | 'error' = $state('idle');
	let invitedUserID: string | null = $state(null);
	let invitedUsername = $state('');
	let inviteError = $state('');

	// ── Online users polling ──────────────────────────────────────────────────────
	let onlineInterval: ReturnType<typeof setInterval> | null = null;

	$effect(() => {
		if (!$authLoading && $user && !onlineInterval) {
			fetchOnlineUsers();
			onlineInterval = setInterval(fetchOnlineUsers, 15_000);
		}
	});

	onDestroy(() => {
		cleanup();
		if (onlineInterval) { clearInterval(onlineInterval); onlineInterval = null; }
	});

	// ── Matchmaking functions ─────────────────────────────────────────────────────

	async function startSearch() {
		if (fi === 'pending') return;
		mm = 'searching';
		waitSeconds = 0;
		errorMsg = '';

		try {
			const res = await fetch(`${API}/api/matchmaking/join`, { method: 'POST', credentials: 'include' });
			if (!res.ok) throw new Error();
		} catch {
			mm = 'error';
			errorMsg = 'Impossibile connettersi al server';
			return;
		}

		waitTimer = setInterval(() => waitSeconds++, 1000);

		eventSource = new EventSource(`${API}/api/matchmaking/stream`, {
			withCredentials: true
		} as EventSourceInit);

		eventSource.addEventListener('connected', () => console.log('SSE matchmaking connesso'));

		eventSource.addEventListener('matched', (e: MessageEvent) => {
			const { game_id } = JSON.parse(e.data);
			mm = 'found';
			cleanup();
			setTimeout(() => goto(`/game/${game_id}`), 1200);
		});

		eventSource.onerror = () => {
			if (mm === 'searching') {
				mm = 'error';
				errorMsg = 'Connessione SSE persa. Riprova.';
				cleanup();
			}
		};
	}

	async function cancelSearch() {
		cleanup();
		await fetch(`${API}/api/matchmaking/leave`, { method: 'DELETE', credentials: 'include' });
		mm = 'idle';
		waitSeconds = 0;
	}

	function cleanup() {
		eventSource?.close();
		eventSource = null;
		if (waitTimer) { clearInterval(waitTimer); waitTimer = null; }
	}

	// ── Friend invite functions ───────────────────────────────────────────────────

	async function handleInvite(targetID: string, targetName: string) {
		if (mm === 'searching') await cancelSearch();
		invitedUserID = targetID;
		invitedUsername = targetName;
		fi = 'pending';
		inviteError = '';
		try {
			await sendInvite(targetID);
			// Il redirect arriva tramite SSE globale del layout (event: matched → friend_match key)
		} catch (err: any) {
			fi = 'error';
			inviteError = err.message ?? 'Errore invio invito';
		}
	}

	function cancelInvite() {
		// L'invito scadrà da solo su Redis (TTL 90s); qui resettiamo solo l'UI
		fi = 'idle';
		invitedUserID = null;
		invitedUsername = '';
	}

	// ── UI helpers ────────────────────────────────────────────────────────────────

	function formatWait(s: number): string {
		const m = Math.floor(s / 60);
		const sec = s % 60;
		return m === 0 ? `${sec}s` : `${m}m ${sec}s`;
	}

	function eloRange(s: number): string {
		if (s < 10) return '±100';
		if (s < 20) return '±200';
		if (s < 30) return '±300';
		if (s < 60) return '±500';
		return 'qualsiasi ELO';
	}

	function eloDiff(opponent: number): string {
		const diff = opponent - ($user?.elo_rapid ?? 1200);
		return diff > 0 ? `+${diff}` : `${diff}`;
	}
</script>

<svelte:head>
	<title>Trova partita — Chess Clone</title>
</svelte:head>

<div class="play-page">

	<!-- ── Invito amico in sospeso ───────────────────────────────────────── -->
	{#if fi === 'pending'}
		<div class="invite-waiting-box">
			<div class="spinner"></div>
			<p class="invite-waiting-text">
				Invito inviato a <strong>{invitedUsername}</strong>
			</p>
			<p class="invite-waiting-sub">In attesa che accetti... (scade in 90s)</p>
			<button class="btn btn-google cancel-btn" onclick={cancelInvite}>
				Annulla invito
			</button>
		</div>

	{:else if fi === 'error'}
		<div class="error-msg" style="max-width:340px;text-align:center">{inviteError}</div>
		<button class="btn btn-primary" onclick={cancelInvite}>OK</button>

	{:else}
		<!-- ── Scheda modalità ────────────────────────────────────────────── -->
		<div class="mode-card">
			<div class="mode-icon">♟</div>
			<h2>Rapid</h2>
			<p class="mode-desc">10 minuti per giocatore</p>
			<p class="mode-elo">Il tuo ELO: <strong>{$user?.elo_rapid ?? '—'}</strong></p>
		</div>

		<!-- ── Matchmaking status ─────────────────────────────────────────── -->
		{#if mm === 'idle'}
			<button class="btn btn-primary play-btn" onclick={startSearch}>
				Trova partita casuale
			</button>

		{:else if mm === 'searching'}
			<div class="searching-box">
				<div class="spinner"></div>
				<p class="wait-time">In attesa... <strong>{formatWait(waitSeconds)}</strong></p>
				<p class="elo-info">Cercando avversario {eloRange(waitSeconds)}</p>
				<button class="btn btn-google cancel-btn" onclick={cancelSearch}>Annulla</button>
			</div>

		{:else if mm === 'found'}
			<div class="found-box">
				<div class="found-icon">⚡</div>
				<p>Match trovato! Avvio partita...</p>
			</div>

		{:else if mm === 'error'}
			<div class="error-msg" style="max-width:340px;text-align:center">{errorMsg}</div>
			<button class="btn btn-primary" onclick={startSearch}>Riprova</button>
		{/if}
	{/if}

	<!-- ── Giocatori online ───────────────────────────────────────────────── -->
	{#if fi === 'idle' && mm !== 'found'}
		<section class="online-section">
			<h3 class="online-title">
				<span class="online-dot"></span>
				Giocatori online
				{#if $onlineUsers.length > 0}
					<span class="online-count">({$onlineUsers.length})</span>
				{/if}
			</h3>

			{#if $onlineUsers.length === 0}
				<p class="online-empty">Nessun altro giocatore online al momento.</p>
			{:else}
				<ul class="online-list">
					{#each $onlineUsers as u (u.id)}
						<li class="online-item">
							<div class="online-item-left">
								<span class="online-avatar">{u.username[0].toUpperCase()}</span>
								<div class="online-item-info">
									<span class="online-item-name">{u.username}</span>
									<span class="online-item-elo">
										ELO {u.elo_rapid}
										<span
											class="elo-diff"
											class:positive={u.elo_rapid > ($user?.elo_rapid ?? 0)}
											class:negative={u.elo_rapid < ($user?.elo_rapid ?? 0)}
										>
											({eloDiff(u.elo_rapid)})
										</span>
									</span>
								</div>
							</div>
							<button
								class="btn btn-primary invite-btn"
								onclick={() => handleInvite(u.id, u.username)}
								disabled={mm === 'searching'}
							>
								Sfida
							</button>
						</li>
					{/each}
				</ul>
			{/if}
		</section>
	{/if}

</div>

<style>
	.play-page {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 1.75rem;
		padding: 3rem 2rem;
	}

	/* ── Mode card ── */
	.mode-card {
		background: var(--bg-card);
		border: 2px solid var(--accent);
		border-radius: 12px;
		padding: 2rem 3rem;
		text-align: center;
		min-width: 240px;
	}
	.mode-icon { font-size: 3rem; margin-bottom: 0.5rem; }
	.mode-card h2 { font-size: 1.8rem; margin-bottom: 0.25rem; }
	.mode-desc { color: var(--text-muted); margin-bottom: 0.5rem; }
	.mode-elo { font-size: 0.95rem; color: var(--text-muted); }

	.play-btn {
		width: 260px;
		padding: 1rem;
		font-size: 1.05rem;
	}

	/* ── Searching ── */
	.searching-box {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.75rem;
	}
	.spinner {
		width: 48px;
		height: 48px;
		border: 4px solid var(--border);
		border-top-color: var(--accent);
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}
	@keyframes spin { to { transform: rotate(360deg); } }
	.wait-time { font-size: 1.1rem; }
	.elo-info { font-size: 0.85rem; color: var(--text-muted); }
	.cancel-btn { width: 180px; margin-top: 0.5rem; }

	/* ── Found ── */
	.found-box {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.75rem;
		animation: fadeIn 0.3s ease;
	}
	.found-icon {
		font-size: 3rem;
		animation: bounce 0.6s ease infinite alternate;
	}
	@keyframes bounce { to { transform: translateY(-8px); } }
	@keyframes fadeIn {
		from { opacity: 0; transform: scale(0.95); }
		to   { opacity: 1; transform: scale(1); }
	}

	/* ── Invite waiting ── */
	.invite-waiting-box {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.6rem;
		background: var(--bg-card);
		border: 2px solid var(--accent);
		border-radius: 12px;
		padding: 2rem 2.5rem;
		text-align: center;
	}
	.invite-waiting-text { font-size: 1.05rem; }
	.invite-waiting-sub { font-size: 0.85rem; color: var(--text-muted); }

	/* ── Online section ── */
	.online-section {
		width: 100%;
		max-width: 480px;
	}
	.online-title {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		font-size: 0.85rem;
		font-weight: 600;
		color: var(--text-muted);
		margin-bottom: 0.75rem;
		text-transform: uppercase;
		letter-spacing: 0.06em;
	}
	.online-dot {
		width: 8px;
		height: 8px;
		background: #2ecc71;
		border-radius: 50%;
		box-shadow: 0 0 6px #2ecc71;
		animation: pulse 2s ease infinite;
		flex-shrink: 0;
	}
	@keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.35; } }
	.online-count { color: var(--text-muted); font-weight: 400; }
	.online-empty {
		color: var(--text-muted);
		font-size: 0.9rem;
		text-align: center;
		padding: 1.5rem 0;
	}
	.online-list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}
	.online-item {
		display: flex;
		align-items: center;
		justify-content: space-between;
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 0.75rem 1rem;
		gap: 0.75rem;
		transition: border-color 0.15s;
	}
	.online-item:hover { border-color: var(--accent); }
	.online-item-left {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		flex: 1;
		min-width: 0;
	}
	.online-avatar {
		width: 36px;
		height: 36px;
		background: var(--accent);
		border-radius: 50%;
		display: flex;
		align-items: center;
		justify-content: center;
		font-weight: 700;
		font-size: 0.95rem;
		flex-shrink: 0;
	}
	.online-item-info {
		display: flex;
		flex-direction: column;
		gap: 0.15rem;
		min-width: 0;
	}
	.online-item-name {
		font-weight: 600;
		font-size: 0.95rem;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.online-item-elo { font-size: 0.8rem; color: var(--text-muted); }
	.elo-diff { font-size: 0.75rem; }
	.elo-diff.positive { color: #2ecc71; }
	.elo-diff.negative { color: #e74c3c; }
	.invite-btn {
		width: auto !important;
		padding: 0.45rem 1rem !important;
		font-size: 0.875rem !important;
		flex-shrink: 0;
		border-radius: 8px !important;
	}
</style>

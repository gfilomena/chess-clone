<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { user as currentUser } from '$lib/stores/auth';
	import { API_URL as API } from '$lib/config';

	const userId = $page.params.id;

	let profile = $state<any>(null);
	let stats = $state<any>(null);
	let games = $state<any[]>([]);
	let loading = $state(true);
	let error = $state('');

	onMount(async () => {
		try {
			const [pRes, sRes, gRes] = await Promise.all([
				fetch(`${API}/api/users/${userId}`),
				fetch(`${API}/api/users/${userId}/stats`),
				fetch(`${API}/api/users/${userId}/games`, { credentials: 'include' })
			]);

			const [p, s, g] = await Promise.all([pRes.json(), sRes.json(), gRes.json()]);

			if (!p.success) throw new Error('Utente non trovato');
			profile = p.data;
			stats = s.data;
			games = g.data ?? [];
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
	});

	function resultForUser(game: any): 'win' | 'loss' | 'draw' {
		if (!game.result) return 'draw';
		const isWhite = game.white_id === userId;
		if (game.result === 'draw') return 'draw';
		if ((game.result === 'white' && isWhite) || (game.result === 'black' && !isWhite)) return 'win';
		return 'loss';
	}

	function opponent(game: any): string {
		return game.white_id === userId ? game.black_username : game.white_username;
	}

	function eloChange(game: any): string {
		const delta = game.elo_after - game.elo_before;
		if (delta === 0) return '—';
		return delta > 0 ? `+${delta}` : `${delta}`;
	}

	function formatDate(str: string | null): string {
		if (!str) return '—';
		return new Date(str).toLocaleDateString('it-IT', {
			day: '2-digit', month: 'short', year: 'numeric'
		});
	}

	function winRate(): string {
		if (!stats || stats.total === 0) return '0';
		return ((stats.wins / stats.total) * 100).toFixed(0);
	}

	const isOwnProfile = $derived($currentUser?.id === userId);
</script>

<svelte:head>
	<title>{profile?.username ?? 'Profilo'} — Chess Clone</title>
</svelte:head>

{#if loading}
	<div style="text-align:center;padding:4rem">
		<p style="color:var(--text-muted)">Caricamento profilo...</p>
	</div>
{:else if error}
	<div style="text-align:center;padding:4rem">
		<p class="error-msg">{error}</p>
	</div>
{:else}
<div class="profile-layout">

	<!-- Header profilo -->
	<div class="profile-header">
		<div class="avatar">
			{#if profile.avatar_url}
				<img src={profile.avatar_url} alt={profile.username} />
			{:else}
				<span>{profile.username[0].toUpperCase()}</span>
			{/if}
		</div>

		<div class="profile-info">
			<h1>{profile.username}</h1>
			<p style="color:var(--text-muted);font-size:0.9rem">
				Membro dal {formatDate(profile.created_at)}
			</p>
		</div>

		{#if isOwnProfile}
			<a href="/play" class="btn btn-primary" style="width:auto;padding:0.6rem 1.2rem">
				♟ Gioca
			</a>
		{/if}
	</div>

	<!-- ELO cards -->
	<div class="elo-cards">
		<div class="elo-card">
			<span class="elo-label">Rapid</span>
			<span class="elo-value">{profile.elo_rapid}</span>
		</div>
		<div class="elo-card">
			<span class="elo-label">Blitz</span>
			<span class="elo-value dimmed">{profile.elo_blitz}</span>
		</div>
		<div class="elo-card">
			<span class="elo-label">Bullet</span>
			<span class="elo-value dimmed">{profile.elo_bullet}</span>
		</div>
	</div>

	<!-- Stats W/L/D -->
	{#if stats}
	<div class="stats-bar">
		<div class="stat-block">
			<span class="stat-num win">{stats.wins}</span>
			<span class="stat-label">Vinte</span>
		</div>
		<div class="stat-block">
			<span class="stat-num draw">{stats.draws}</span>
			<span class="stat-label">Patte</span>
		</div>
		<div class="stat-block">
			<span class="stat-num loss">{stats.losses}</span>
			<span class="stat-label">Perse</span>
		</div>
		<div class="stat-block">
			<span class="stat-num">{stats.total}</span>
			<span class="stat-label">Totali</span>
		</div>
		<div class="stat-block">
			<span class="stat-num accent">{winRate()}%</span>
			<span class="stat-label">Win rate</span>
		</div>
	</div>

	<!-- Barra grafica W/L/D -->
	{#if stats.total > 0}
	<div class="wld-bar">
		<div class="wld-win"  style="width:{(stats.wins/stats.total)*100}%"   title="Vinte"></div>
		<div class="wld-draw" style="width:{(stats.draws/stats.total)*100}%"  title="Patte"></div>
		<div class="wld-loss" style="width:{(stats.losses/stats.total)*100}%" title="Perse"></div>
	</div>
	{/if}
	{/if}

	<!-- Storico partite -->
	<div class="games-section">
		<h2>Ultime partite</h2>
		{#if games.length === 0}
			<p style="color:var(--text-muted)">Nessuna partita giocata.</p>
		{:else}
			<table class="games-table">
				<thead>
					<tr>
						<th>Risultato</th>
						<th>Avversario</th>
						<th>Motivo</th>
						<th>ELO</th>
						<th>Data</th>
						<th></th>
					</tr>
				</thead>
				<tbody>
					{#each games as game}
						{@const res = resultForUser(game)}
						<tr>
							<td>
								<span class="result-badge {res}">
									{res === 'win' ? 'V' : res === 'loss' ? 'S' : 'P'}
								</span>
							</td>
							<td>{opponent(game)}</td>
							<td style="color:var(--text-muted);font-size:0.85rem">
								{game.finish_reason ?? '—'}
							</td>
							<td class="elo-delta" class:positive={game.elo_after > game.elo_before} class:negative={game.elo_after < game.elo_before}>
								{eloChange(game)}
							</td>
							<td style="color:var(--text-muted);font-size:0.85rem">
								{formatDate(game.finished_at)}
							</td>
							<td>
								<a href="/analysis/{game.id}" style="font-size:0.8rem;color:var(--accent)">
									Analizza
								</a>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
	</div>

</div>
{/if}

<style>
	.profile-layout {
		max-width: 800px;
		margin: 0 auto;
		padding: 2rem;
		display: flex;
		flex-direction: column;
		gap: 1.5rem;
	}

	/* Header */
	.profile-header {
		display: flex;
		align-items: center;
		gap: 1.5rem;
	}
	.avatar {
		width: 72px;
		height: 72px;
		border-radius: 50%;
		background: var(--bg-input);
		border: 2px solid var(--accent);
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 2rem;
		font-weight: 700;
		overflow: hidden;
		flex-shrink: 0;
	}
	.avatar img { width: 100%; height: 100%; object-fit: cover; }
	.profile-info h1 { font-size: 1.8rem; }
	.profile-info { flex: 1; }

	/* ELO cards */
	.elo-cards {
		display: flex;
		gap: 1rem;
	}
	.elo-card {
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 1rem 1.5rem;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.25rem;
		flex: 1;
	}
	.elo-label { font-size: 0.8rem; color: var(--text-muted); text-transform: uppercase; }
	.elo-value { font-size: 1.8rem; font-weight: 700; }
	.elo-value.dimmed { color: var(--text-muted); font-size: 1.4rem; }

	/* Stats */
	.stats-bar {
		display: flex;
		gap: 2rem;
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 1rem 1.5rem;
	}
	.stat-block {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.2rem;
	}
	.stat-num { font-size: 1.6rem; font-weight: 700; }
	.stat-num.win    { color: var(--accent); }
	.stat-num.draw   { color: #94a3b8; }
	.stat-num.loss   { color: var(--danger); }
	.stat-num.accent { color: var(--accent); }
	.stat-label { font-size: 0.75rem; color: var(--text-muted); }

	/* W/L/D bar */
	.wld-bar {
		display: flex;
		height: 8px;
		border-radius: 4px;
		overflow: hidden;
		background: var(--border);
	}
	.wld-win  { background: var(--accent); transition: width 0.5s; }
	.wld-draw { background: #94a3b8; transition: width 0.5s; }
	.wld-loss { background: var(--danger); transition: width 0.5s; }

	/* Games table */
	.games-section h2 {
		font-size: 1.1rem;
		margin-bottom: 0.75rem;
		color: var(--text-muted);
	}
	.games-table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.9rem;
	}
	.games-table th {
		text-align: left;
		padding: 0.5rem 0.75rem;
		color: var(--text-muted);
		font-size: 0.75rem;
		text-transform: uppercase;
		border-bottom: 1px solid var(--border);
	}
	.games-table td {
		padding: 0.6rem 0.75rem;
		border-bottom: 1px solid var(--border);
		vertical-align: middle;
	}
	.games-table tr:hover td { background: var(--bg-card); }

	.result-badge {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		border-radius: 50%;
		font-weight: 700;
		font-size: 0.8rem;
	}
	.result-badge.win  { background: rgba(74, 222, 128, 0.15); color: var(--accent); border: 1px solid var(--accent); }
	.result-badge.loss { background: rgba(248,113,113,0.15); color: var(--danger); border: 1px solid var(--danger); }
	.result-badge.draw { background: var(--bg-input); color: var(--text-muted); border: 1px solid var(--border); }

	.elo-delta { font-weight: 600; }
	.elo-delta.positive { color: var(--accent); }
	.elo-delta.negative { color: var(--danger); }
</style>

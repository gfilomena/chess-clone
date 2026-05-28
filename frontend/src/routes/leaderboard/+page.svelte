<script lang="ts">
	import { onMount } from 'svelte';
	import { API_URL } from '$lib/config';
	import { user } from '$lib/stores/auth';
	import { t } from '$lib/i18n';

	interface Entry {
		id: string;
		username: string;
		avatar_url: string | null;
		elo_rapid: number;
		total_games: number;
	}

	let entries = $state<Entry[]>([]);
	let loading  = $state(true);
	let error    = $state<string | null>(null);

	onMount(async () => {
		try {
			const res = await fetch(`${API_URL}/api/leaderboard`, { credentials: 'include' });
			if (!res.ok) throw new Error('server error');
			entries = await res.json();
		} catch {
			error = 'err';
		} finally {
			loading = false;
		}
	});

	function initial(name: string) {
		return name[0]?.toUpperCase() ?? '?';
	}

	function rankIcon(i: number) {
		if (i === 0) return '🥇';
		if (i === 1) return '🥈';
		if (i === 2) return '🥉';
		return `${i + 1}`;
	}
</script>

<svelte:head>
	<title>Chess</title>
</svelte:head>

<div class="page">
	<div class="header">
		<h1>{$t.leaderboard.title}</h1>
		<p class="sub">{$t.leaderboard.sub}</p>
	</div>

	{#if loading}
		<div class="state-msg">{$t.leaderboard.loading}</div>
	{:else if error}
		<div class="state-msg error">{$t.leaderboard.error}</div>
	{:else if entries.length === 0}
		<div class="state-msg">{$t.leaderboard.empty}</div>
	{:else}
		<div class="table-wrap">
			<table>
				<thead>
					<tr>
						<th class="col-rank">{$t.leaderboard.rank}</th>
						<th class="col-player">{$t.leaderboard.player}</th>
						<th class="col-elo">{$t.leaderboard.elo}</th>
						<th class="col-games">{$t.leaderboard.games}</th>
					</tr>
				</thead>
				<tbody>
					{#each entries as entry, i}
						{@const isMe = $user?.id === entry.id}
						<tr class:me={isMe} class:podium={i < 3}>
							<td class="col-rank">
								<span class="rank" class:medal={i < 3}>{rankIcon(i)}</span>
							</td>
							<td class="col-player">
								<a href="/profile/{entry.id}" class="player-link">
									<div class="avatar" class:me-avatar={isMe}>
										{initial(entry.username)}
									</div>
									<span class="username" class:me-name={isMe}>
										{entry.username}
										{#if isMe}<span class="you-tag">{$t.leaderboard.you}</span>{/if}
									</span>
								</a>
							</td>
							<td class="col-elo">
								<span class="elo" class:top={i === 0}>{entry.elo_rapid}</span>
							</td>
							<td class="col-games">{entry.total_games}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</div>

<style>
.page {
	max-width: 680px;
	margin: 0 auto;
	padding: 2rem 1.5rem 4rem;
	display: flex;
	flex-direction: column;
	gap: 1.5rem;
}

.header {
	text-align: center;
}
.header h1 {
	font-size: 2rem;
	font-weight: 800;
	margin-bottom: 0.3rem;
}
.sub {
	color: var(--text-muted);
	font-size: 0.9rem;
}

.state-msg {
	text-align: center;
	color: var(--text-muted);
	padding: 3rem 0;
	font-size: 1rem;
}
.state-msg.error { color: var(--danger, #e05050); }

/* ── Table ── */
.table-wrap {
	background: var(--bg-card);
	border: 1px solid var(--border);
	border-radius: 12px;
	overflow: hidden;
}

table {
	width: 100%;
	border-collapse: collapse;
}

thead tr {
	border-bottom: 1px solid var(--border);
}
thead th {
	padding: 0.75rem 1rem;
	font-size: 0.72rem;
	text-transform: uppercase;
	letter-spacing: 0.06em;
	color: var(--text-muted);
	font-weight: 600;
	text-align: left;
}

tbody tr {
	border-bottom: 1px solid var(--border);
	transition: background 0.12s;
}
tbody tr:last-child { border-bottom: none; }
tbody tr:hover { background: rgba(255,255,255,0.03); }
tbody tr.me { background: color-mix(in srgb, var(--accent) 8%, transparent); }
tbody tr.me:hover { background: color-mix(in srgb, var(--accent) 12%, transparent); }

td {
	padding: 0.65rem 1rem;
	vertical-align: middle;
}

/* ── Columns ── */
.col-rank  { width: 3.5rem; text-align: center; }
.col-elo   { width: 6rem;   text-align: right; }
.col-games { width: 5rem;   text-align: right; color: var(--text-muted); font-size: 0.85rem; }

.rank {
	font-size: 0.85rem;
	font-weight: 700;
	color: var(--text-muted);
}
.rank.medal {
	font-size: 1.15rem;
}

/* ── Player cell ── */
.player-link {
	display: flex;
	align-items: center;
	gap: 0.65rem;
	text-decoration: none;
	color: var(--text);
}
.player-link:hover .username { color: var(--accent); }

.avatar {
	width: 32px;
	height: 32px;
	border-radius: 50%;
	background: var(--border);
	display: flex;
	align-items: center;
	justify-content: center;
	font-size: 0.8rem;
	font-weight: 700;
	color: var(--text-muted);
	flex-shrink: 0;
}
.avatar.me-avatar {
	background: var(--accent);
	color: #000;
}

.username {
	font-weight: 500;
	font-size: 0.95rem;
	transition: color 0.15s;
}
.username.me-name { font-weight: 700; }

.you-tag {
	display: inline-block;
	font-size: 0.58rem;
	font-weight: 700;
	text-transform: uppercase;
	letter-spacing: 0.05em;
	background: var(--accent);
	color: #000;
	border-radius: 20px;
	padding: 0.08rem 0.4rem;
	margin-left: 0.35rem;
	vertical-align: middle;
}

/* ── ELO ── */
.elo {
	font-weight: 700;
	font-size: 0.95rem;
	font-variant-numeric: tabular-nums;
}
.elo.top { color: var(--accent); font-size: 1.05rem; }

/* ── Mobile ── */
@media (max-width: 768px) {
	.page { padding: 1.25rem 0.75rem 3rem; }
	.header h1 { font-size: 1.6rem; }
	.col-games { display: none; }
	td, th { padding: 0.55rem 0.65rem; }
}
</style>

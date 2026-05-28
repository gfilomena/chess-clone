<script lang="ts">
	import { user, authLoading } from '$lib/stores/auth';
</script>

<svelte:head>
	<title>Chess Clone</title>
</svelte:head>

<main style="text-align:center; padding: 4rem 2rem;">
	{#if $authLoading}
		<p style="color: var(--text-muted)">Caricamento...</p>
	{:else if $user}
		<h1 style="font-size:2.5rem; margin-bottom:0.5rem">
			Bentornato, {$user.username}!
		</h1>
		<p style="color: var(--text-muted); margin-bottom: 2rem">
			ELO Rapid: <strong style="color:var(--accent)">{$user.elo_rapid}</strong>
		</p>
		<a href="/play" class="btn btn-primary" style="width:auto; padding: 0.8rem 2rem; font-size:1.1rem">
			♟ Gioca una partita
		</a>
	{:else}
		<h1 style="font-size:3rem; margin-bottom:1rem">♟ Chess Clone</h1>
		<p style="color: var(--text-muted); margin-bottom: 2rem; font-size:1.1rem">
			Gioca agli scacchi online. Rapid 10 minuti.
		</p>
		<div style="display:flex; gap:1rem; justify-content:center; flex-wrap:wrap">
			<a href="/register" class="btn btn-primary" style="width:auto; padding: 0.8rem 2rem">
				Inizia a giocare
			</a>
			<a href="/login" class="btn btn-google" style="width:auto; padding: 0.8rem 2rem">
				Hai già un account
			</a>
		</div>
	{/if}
</main>

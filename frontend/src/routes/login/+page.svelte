<script lang="ts">
	import { login, devLogin, user, authLoading } from '$lib/stores/auth';
	import { goto } from '$app/navigation';
	import { API_URL, DEV_MODE } from '$lib/config';

	// Se già autenticato → vai alla home
	$effect(() => {
		if (!$authLoading && $user) goto('/');
	});

	// ── Normal login state ────────────────────────────────────────────────────────
	let email = $state('');
	let password = $state('');

	// ── Dev login state ───────────────────────────────────────────────────────────
	let devUsername = $state('');

	// ── Shared ───────────────────────────────────────────────────────────────────
	let error = $state('');
	let loading = $state(false);

	async function handleLogin(e: Event) {
		e.preventDefault();
		error = '';
		loading = true;
		try {
			await login(email, password);
			goto('/');
		} catch (err: any) {
			error = err.message ?? 'Errore durante il login';
		} finally {
			loading = false;
		}
	}

	async function handleDevLogin(e: Event) {
		e.preventDefault();
		error = '';
		loading = true;
		try {
			await devLogin(devUsername.trim());
			goto('/');
		} catch (err: any) {
			error = err.message ?? 'Utente non trovato';
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Accedi — Chess</title>
</svelte:head>

{#if $authLoading}
	<!-- Spinner mentre verifica la sessione — evita il flash del form -->
	<div class="auth-checking">
		<div class="auth-spinner"></div>
	</div>

{:else}
	<div class="form-card">
		<h1>Accedi</h1>

		{#if error}
			<div class="error-msg">{error}</div>
		{/if}

		{#if DEV_MODE}
			<!-- ── Banner DEV ──────────────────────────────────────────────── -->
			<div class="dev-banner">
				<span class="dev-badge">DEV</span>
				Login rapido — solo username
			</div>

			<form onsubmit={handleDevLogin}>
				<div class="field">
					<label for="dev-username">Username</label>
					<input
						id="dev-username"
						type="text"
						bind:value={devUsername}
						placeholder="il_tuo_username"
						required
						autocomplete="username"
					/>
				</div>

				<button class="btn btn-primary" type="submit" disabled={loading || !devUsername.trim()}>
					{loading ? 'Accesso...' : '⚡ Accedi (DEV)'}
				</button>
			</form>

			<div class="divider">oppure login completo</div>
		{/if}

		<!-- ── Login normale ───────────────────────────────────────────────── -->
		<form onsubmit={handleLogin}>
			<div class="field">
				<label for="email">Email</label>
				<input
					id="email"
					type="email"
					bind:value={email}
					placeholder="tuaemail@esempio.com"
					required
					autocomplete="email"
				/>
			</div>

			<div class="field">
				<label for="password">Password</label>
				<input
					id="password"
					type="password"
					bind:value={password}
					placeholder="••••••••"
					required
					autocomplete="current-password"
				/>
			</div>

			<button class="btn btn-primary" type="submit" disabled={loading}>
				{loading ? 'Accesso in corso...' : 'Accedi'}
			</button>
		</form>

		<div class="divider">oppure</div>

		<a href="{API_URL}/api/auth/google" class="btn btn-google">
			<svg width="18" height="18" viewBox="0 0 48 48">
				<path fill="#EA4335" d="M24 9.5c3.54 0 6.71 1.22 9.21 3.6l6.85-6.85C35.9 2.38 30.47 0 24 0 14.62 0 6.51 5.38 2.56 13.22l7.98 6.19C12.43 13.72 17.74 9.5 24 9.5z"/>
				<path fill="#4285F4" d="M46.98 24.55c0-1.57-.15-3.09-.38-4.55H24v9.02h12.94c-.58 2.96-2.26 5.48-4.78 7.18l7.73 6c4.51-4.18 7.09-10.36 7.09-17.65z"/>
				<path fill="#FBBC05" d="M10.53 28.59c-.48-1.45-.76-2.99-.76-4.59s.27-3.14.76-4.59l-7.98-6.19C.92 16.46 0 20.12 0 24c0 3.88.92 7.54 2.56 10.78l7.97-6.19z"/>
				<path fill="#34A853" d="M24 48c6.48 0 11.93-2.13 15.89-5.81l-7.73-6c-2.18 1.48-4.97 2.31-8.16 2.31-6.26 0-11.57-4.22-13.47-9.91l-7.98 6.19C6.51 42.62 14.62 48 24 48z"/>
			</svg>
			Continua con Google
		</a>

		<p class="form-footer">
			Non hai un account? <a href="/register">Registrati</a>
		</p>
	</div>
{/if}

<style>
	/* ── Auth checking ── */
	.auth-checking {
		display: flex;
		justify-content: center;
		align-items: center;
		padding: 8rem 0;
	}

	.auth-spinner {
		width: 36px;
		height: 36px;
		border: 3px solid var(--border);
		border-top-color: var(--accent);
		border-radius: 50%;
		animation: spin 0.7s linear infinite;
	}

	@keyframes spin { to { transform: rotate(360deg); } }

	/* ── Dev banner ── */
	.dev-banner {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		background: rgba(255, 193, 7, 0.12);
		border: 1px solid rgba(255, 193, 7, 0.4);
		border-radius: 8px;
		padding: 0.6rem 0.9rem;
		font-size: 0.85rem;
		color: #ffc107;
		margin-bottom: 0.25rem;
	}

	.dev-badge {
		background: #ffc107;
		color: #000;
		font-weight: 800;
		font-size: 0.7rem;
		padding: 0.15rem 0.4rem;
		border-radius: 4px;
		letter-spacing: 0.05em;
	}
</style>

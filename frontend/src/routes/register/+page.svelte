<script lang="ts">
	import { register, user, authLoading } from '$lib/stores/auth';
	import { goto } from '$app/navigation';
	import { API_URL } from '$lib/config';

	// Se già autenticato → vai alla home
	$effect(() => {
		if (!$authLoading && $user) {
			goto('/');
		}
	});

	let username = $state('');
	let email = $state('');
	let password = $state('');
	let confirm = $state('');
	let error = $state('');
	let loading = $state(false);

	async function handleRegister(e: Event) {
		e.preventDefault();
		error = '';

		if (password !== confirm) {
			error = 'Le password non coincidono';
			return;
		}
		if (password.length < 8) {
			error = 'La password deve avere almeno 8 caratteri';
			return;
		}

		loading = true;
		try {
			await register(username, email, password);
			goto('/');
		} catch (err: any) {
			error = err.message ?? 'Errore durante la registrazione';
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Registrati — Chess Clone</title>
</svelte:head>

{#if $authLoading}
	<div class="auth-checking">
		<div class="auth-spinner"></div>
	</div>
{:else}
<div class="form-card">
	<h1>Crea account</h1>

	{#if error}
		<div class="error-msg">{error}</div>
	{/if}

	<form onsubmit={handleRegister}>
		<div class="field">
			<label for="username">Username</label>
			<input
				id="username"
				type="text"
				bind:value={username}
				placeholder="il_tuo_username"
				required
				minlength="3"
				maxlength="30"
				autocomplete="username"
			/>
		</div>

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
				placeholder="min. 8 caratteri"
				required
				minlength="8"
				autocomplete="new-password"
			/>
		</div>

		<div class="field">
			<label for="confirm">Conferma password</label>
			<input
				id="confirm"
				type="password"
				bind:value={confirm}
				placeholder="••••••••"
				required
				autocomplete="new-password"
			/>
		</div>

		<button class="btn btn-primary" type="submit" disabled={loading}>
			{loading ? 'Registrazione...' : 'Crea account'}
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
		Hai già un account? <a href="/login">Accedi</a>
	</p>
</div>
{/if}

<style>
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
</style>

<script lang="ts">
	import { login, devLogin, user, authLoading, resendVerification } from '$lib/stores/auth';
	import { goto } from '$app/navigation';
	import { API_URL, DEV_MODE } from '$lib/config';
	import { t } from '$lib/i18n';

	// Se già autenticato → vai alla home
	$effect(() => {
		if (!$authLoading && $user) goto('/');
	});

	// ── Normal login state ────────────────────────────────────────────────────────
	let email    = $state('');
	let password = $state('');

	// ── Dev login state ───────────────────────────────────────────────────────────
	let devUsername = $state('');

	// ── Shared ───────────────────────────────────────────────────────────────────
	let error   = $state('');
	let loading = $state(false);

	// ── Email not verified state ──────────────────────────────────────────────────
	let notVerified     = $state(false);
	let resendLoading   = $state(false);
	let resendDone      = $state(false);

	async function handleLogin(e: Event) {
		e.preventDefault();
		error = '';
		notVerified = false;
		loading = true;
		try {
			await login(email, password);
			goto('/');
		} catch (err: any) {
			const code: string = err.message ?? '';
			if (code === 'EMAIL_NOT_VERIFIED') {
				notVerified = true;
			} else if (code === 'INVALID_CREDENTIALS') {
				error = $t.auth.err_login;
			} else if (code === 'BANNED') {
				error = 'Account sospeso.';
			} else {
				error = $t.auth.err_login;
			}
		} finally {
			loading = false;
		}
	}

	async function handleResend() {
		resendLoading = true;
		resendDone = false;
		try {
			await resendVerification(email);
			resendDone = true;
		} catch {
			resendDone = true;
		} finally {
			resendLoading = false;
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
	<title>Chess</title>
</svelte:head>

{#if $authLoading}
	<div class="auth-checking">
		<div class="auth-spinner"></div>
	</div>

{:else}
	<div class="form-card">
		<h1>{$t.auth.login_title}</h1>

		{#if error}
			<div class="error-msg">{error}</div>
		{/if}

		<!-- Email non verificata ─────────────────────────────────────────── -->
		{#if notVerified}
			<div class="not-verified-banner">
				<p class="nv-msg">{$t.auth.err_not_verified}</p>
				{#if resendDone}
					<span class="resent-ok">{$t.auth.verify_resent}</span>
				{:else}
					<button
						class="btn-resend"
						onclick={handleResend}
						disabled={resendLoading}
					>
						{resendLoading ? $t.auth.verify_resending : $t.auth.verify_resend}
					</button>
				{/if}
			</div>
		{/if}

		{#if DEV_MODE}
			<div class="dev-banner">
				<span class="dev-badge">DEV</span>
				{$t.auth.dev_quick}
			</div>

			<form onsubmit={handleDevLogin}>
				<div class="field">
					<label for="dev-username">{$t.auth.username}</label>
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
					{loading ? $t.auth.login_loading : $t.auth.dev_btn}
				</button>
			</form>

			<div class="divider">{$t.auth.dev_or}</div>
		{/if}

		<form onsubmit={handleLogin}>
			<div class="field">
				<label for="email">{$t.auth.email}</label>
				<input
					id="email"
					type="email"
					bind:value={email}
					placeholder={$t.auth.email_placeholder}
					required
					autocomplete="email"
				/>
			</div>

			<div class="field">
				<label for="password">{$t.auth.password}</label>
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
				{loading ? $t.auth.login_loading : $t.auth.login_btn}
			</button>
		</form>

		<div class="divider">{$t.auth.or}</div>

		<a href="{API_URL}/api/auth/google" class="btn btn-google">
			<svg width="18" height="18" viewBox="0 0 48 48">
				<path fill="#EA4335" d="M24 9.5c3.54 0 6.71 1.22 9.21 3.6l6.85-6.85C35.9 2.38 30.47 0 24 0 14.62 0 6.51 5.38 2.56 13.22l7.98 6.19C12.43 13.72 17.74 9.5 24 9.5z"/>
				<path fill="#4285F4" d="M46.98 24.55c0-1.57-.15-3.09-.38-4.55H24v9.02h12.94c-.58 2.96-2.26 5.48-4.78 7.18l7.73 6c4.51-4.18 7.09-10.36 7.09-17.65z"/>
				<path fill="#FBBC05" d="M10.53 28.59c-.48-1.45-.76-2.99-.76-4.59s.27-3.14.76-4.59l-7.98-6.19C.92 16.46 0 20.12 0 24c0 3.88.92 7.54 2.56 10.78l7.97-6.19z"/>
				<path fill="#34A853" d="M24 48c6.48 0 11.93-2.13 15.89-5.81l-7.73-6c-2.18 1.48-4.97 2.31-8.16 2.31-6.26 0-11.57-4.22-13.47-9.91l-7.98 6.19C6.51 42.62 14.62 48 24 48z"/>
			</svg>
			{$t.auth.google}
		</a>

		<p class="form-footer">
			{$t.auth.no_account} <a href="/register">{$t.auth.sign_up}</a>
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

	/* ── Email not verified ── */
	.not-verified-banner {
		background: rgba(255, 152, 0, 0.1);
		border: 1px solid rgba(255, 152, 0, 0.4);
		border-radius: 10px;
		padding: 0.9rem 1rem;
		margin-bottom: 0.75rem;
		display: flex;
		flex-direction: column;
		gap: 0.6rem;
	}

	.nv-msg {
		margin: 0;
		font-size: 0.88rem;
		color: #ff9800;
		line-height: 1.4;
	}

	.btn-resend {
		align-self: flex-start;
		background: transparent;
		border: 1px solid rgba(255, 152, 0, 0.5);
		color: #ff9800;
		padding: 0.4rem 0.9rem;
		border-radius: 6px;
		cursor: pointer;
		font-size: 0.82rem;
		transition: background 0.15s;
	}

	.btn-resend:hover:not(:disabled) {
		background: rgba(255, 152, 0, 0.15);
	}

	.btn-resend:disabled {
		opacity: 0.5;
		cursor: default;
	}

	.resent-ok {
		font-size: 0.85rem;
		color: var(--accent);
		font-weight: 600;
	}
</style>

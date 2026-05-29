<svelte:head>
	<title>Privacy Policy — Chess</title>
	<meta name="description" content="Informativa sulla privacy e sui cookie di Chess, piattaforma di scacchi online gratuita." />
</svelte:head>

<div class="legal-page">
	<div class="legal-card">

		<h1>Privacy Policy</h1>
		<p class="updated">Ultimo aggiornamento: maggio 2026</p>

		<section>
			<h2>1. Titolare del trattamento</h2>
			<p>
				Il titolare del trattamento dei dati è il gestore del sito Chess, contattabile all'indirizzo
				e-mail: <a href="mailto:salsx@hotmail.it">salsx@hotmail.it</a>.
			</p>
		</section>

		<section>
			<h2>2. Dati raccolti</h2>
			<p>Raccogliamo le seguenti categorie di dati:</p>
			<ul>
				<li><strong>Dati di registrazione</strong>: username, indirizzo e-mail e password (conservata in forma hash).</li>
				<li><strong>Dati di gioco</strong>: partite giocate, mosse, risultati, rating ELO.</li>
				<li><strong>Dati tecnici</strong>: indirizzo IP, tipo di browser, sistema operativo (log di accesso standard del server).</li>
				<li><strong>Cookie</strong>: vedi la sezione 4.</li>
			</ul>
		</section>

		<section>
			<h2>3. Finalità del trattamento</h2>
			<p>I dati sono trattati per:</p>
			<ul>
				<li>Fornire e migliorare il servizio di scacchi online (base giuridica: esecuzione del contratto).</li>
				<li>Garantire la sicurezza e prevenire abusi (base giuridica: legittimo interesse).</li>
				<li>Mostrare pubblicità contestuale tramite Google AdSense (base giuridica: consenso).</li>
			</ul>
		</section>

		<section>
			<h2>4. Cookie</h2>
			<p>Utilizziamo le seguenti categorie di cookie:</p>

			<h3>Cookie tecnici / essenziali</h3>
			<p>
				Necessari al funzionamento del sito. Non richiedono consenso. Includono il cookie di sessione
				per l'autenticazione (<code>auth_token</code>) e la preferenza di lingua (<code>lang</code>).
			</p>

			<h3>Cookie pubblicitari (solo con consenso)</h3>
			<p>
				Se accetti tutti i cookie, carichiamo lo script di <strong>Google AdSense</strong>
				(<code>pagead2.googlesyndication.com</code>), che imposta cookie propri per mostrare annunci
				personalizzati. Puoi gestire le preferenze pubblicitarie di Google su
				<a href="https://adssettings.google.com" target="_blank" rel="noopener noreferrer">adssettings.google.com</a>.
			</p>

			<h3>Gestione del consenso</h3>
			<p>
				La tua scelta è salvata nel browser (<code>localStorage.cookie_consent</code>).
				Puoi revocarla in qualsiasi momento svuotando i dati del sito dalle impostazioni del browser.
			</p>
		</section>

		<section>
			<h2>5. Terze parti</h2>
			<ul>
				<li>
					<strong>Google AdSense / Google LLC</strong> — pubblicità contestuale.
					<a href="https://policies.google.com/privacy" target="_blank" rel="noopener noreferrer">Privacy policy Google</a>.
				</li>
				<li>
					<strong>Provider di hosting</strong> — il server che ospita il sito può registrare log di accesso standard.
				</li>
			</ul>
		</section>

		<section>
			<h2>6. Conservazione dei dati</h2>
			<p>
				I dati di account vengono conservati fino alla cancellazione del profilo.
				I log del server sono conservati per un massimo di 30 giorni.
			</p>
		</section>

		<section>
			<h2>7. Diritti dell'utente (GDPR)</h2>
			<p>Hai il diritto di:</p>
			<ul>
				<li>accedere ai tuoi dati personali;</li>
				<li>richiederne la rettifica o la cancellazione;</li>
				<li>opporti al trattamento o richiederne la limitazione;</li>
				<li>ricevere i tuoi dati in formato portabile;</li>
				<li>revocare il consenso in qualsiasi momento (senza pregiudicare la liceità del trattamento precedente);</li>
				<li>proporre reclamo all'autorità di controllo (Garante per la protezione dei dati personali — <a href="https://www.garanteprivacy.it" target="_blank" rel="noopener noreferrer">garanteprivacy.it</a>).</li>
			</ul>
			<p>
				Per esercitare i tuoi diritti scrivi a: <a href="mailto:salsx@hotmail.it">salsx@hotmail.it</a>.
			</p>
		</section>

		<section>
			<h2>8. Modifiche alla presente informativa</h2>
			<p>
				Ci riserviamo di aggiornare questa informativa in caso di variazioni del servizio o della
				normativa applicabile. La data di ultimo aggiornamento è sempre indicata in cima alla pagina.
			</p>
		</section>

	</div>
</div>

<style>
	.legal-page {
		max-width: 720px;
		margin: 0 auto;
		padding: 2rem 1.5rem 4rem;
	}

	.legal-card {
		background: var(--bg-card);
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 2.5rem 2rem;
	}

	h1 {
		font-size: 1.75rem;
		font-weight: 800;
		color: var(--text);
		margin-bottom: 0.25rem;
	}

	.updated {
		font-size: 0.82rem;
		color: var(--text-muted);
		margin-bottom: 2rem;
	}

	section {
		margin-bottom: 2rem;
	}

	h2 {
		font-size: 1.05rem;
		font-weight: 700;
		color: var(--accent);
		margin-bottom: 0.6rem;
		border-bottom: 1px solid var(--border);
		padding-bottom: 0.3rem;
	}

	h3 {
		font-size: 0.95rem;
		font-weight: 600;
		color: var(--text);
		margin: 1rem 0 0.4rem;
	}

	p {
		font-size: 0.9rem;
		color: var(--text-muted);
		line-height: 1.7;
		margin-bottom: 0.5rem;
	}

	ul {
		padding-left: 1.4rem;
		margin-bottom: 0.5rem;
	}

	li {
		font-size: 0.9rem;
		color: var(--text-muted);
		line-height: 1.7;
		margin-bottom: 0.2rem;
	}

	strong { color: var(--text); }

	a { color: var(--accent); }
	a:hover { text-decoration: underline; }

	code {
		font-size: 0.82rem;
		background: var(--bg-input);
		border-radius: 3px;
		padding: 0.1em 0.35em;
		color: var(--text);
	}

	@media (max-width: 768px) {
		.legal-card { padding: 1.5rem 1rem; }
		h1 { font-size: 1.4rem; }
	}
</style>

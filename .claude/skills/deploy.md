# Skill: deploy

Esegui il deploy dell'app chess-clone su Railway e mostra lo stato finale.

## Procedura

1. **Controlla lo stato git**
   ```bash
   git status
   git diff --stat
   ```
   Se ci sono modifiche non committate, esegui commit autonomo con messaggio descrittivo (segui le istruzioni globali per i commit). Non chiedere conferma per operazioni ordinarie.

2. **Push su GitHub** (triggera l'auto-deploy Railway)
   ```bash
   git push origin main
   ```

3. **Monitora il deployment Railway**
   Usa `railway status` per vedere lo stato corrente del deployment.
   ```bash
   railway status
   ```
   Poi leggi i log del deployment più recente:
   ```bash
   railway logs --tail 50
   ```

4. **Verifica health check**
   Aspetta che l'app risponda su `/health`:
   ```bash
   curl -s -o /dev/null -w "%{http_code}" https://$(railway domain)/health 2>/dev/null || echo "checking..."
   ```
   Ripeti ogni 10 secondi finché restituisce `200` (max 3 minuti).

5. **Mostra il risultato finale**

   Se il deploy ha avuto successo (health check 200), stampa SEMPRE a video:

   ```
   🟢 Deploy completato — chess-clone è live
   ```
   
   Seguito da:
   - URL dell'app (da `railway domain`)
   - Commit deployato (`git log -1 --oneline`)
   - Timestamp

   Se il deploy fallisce, stampa:
   ```
   🔴 Deploy fallito
   ```
   Seguito dai log di errore rilevanti da `railway logs`.

## Note

- Il progetto usa Railway con Dockerfile multi-stage (frontend SvelteKit + backend Go).
- Il build dura circa 3-5 minuti la prima volta, 1-2 minuti con cache.
- Il health check è su `GET /health` — risponde 200 quando il server Go è pronto.
- Se `railway` CLI non è autenticato, mostra il comando `railway login` e fermati.

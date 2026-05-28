package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"chess-clone/backend/internal/db"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type OAuthHandler struct {
	pg     *db.Postgres
	config *oauth2.Config
}

// Stato temporaneo anti-CSRF
const oauthStateValue = "chess_oauth_state"

func NewOAuthHandler(pg *db.Postgres) *OAuthHandler {
	config := &oauth2.Config{
		ClientID:     getEnvOAuth("GOOGLE_CLIENT_ID"),
		ClientSecret: getEnvOAuth("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  getEnvOAuth("GOOGLE_REDIRECT_URL"),
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}
	return &OAuthHandler{pg: pg, config: config}
}

// GET /api/auth/google
// Redirige l'utente alla pagina di login di Google
func (h *OAuthHandler) RedirectToGoogle(w http.ResponseWriter, r *http.Request) {
	url := h.config.AuthCodeURL(oauthStateValue, oauth2.AccessTypeOnline)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// GET /api/auth/google/callback
// Google redirige qui dopo il login
func (h *OAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	// Verifica stato anti-CSRF
	if r.URL.Query().Get("state") != oauthStateValue {
		http.Error(w, "stato OAuth non valido", http.StatusBadRequest)
		return
	}

	// Scambia il codice con il token
	code := r.URL.Query().Get("code")
	token, err := h.config.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, "scambio token fallito", http.StatusInternalServerError)
		return
	}

	// Recupera info utente da Google
	googleUser, err := getGoogleUserInfo(token.AccessToken)
	if err != nil {
		http.Error(w, "recupero profilo Google fallito", http.StatusInternalServerError)
		return
	}

	// Cerca o crea l'utente nel DB
	userID, err := h.findOrCreateGoogleUser(r.Context(), googleUser)
	if err != nil {
		http.Error(w, "errore database", http.StatusInternalServerError)
		return
	}

	// Genera JWT e imposta cookie
	jwtToken, err := generateJWT(userID)
	if err != nil {
		http.Error(w, "errore generazione token", http.StatusInternalServerError)
		return
	}

	setAuthCookie(w, jwtToken)

	// Redirige al frontend (legge FRONTEND_URL da .env)
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5174"
	}
	http.Redirect(w, r, frontendURL, http.StatusTemporaryRedirect)
}

// Struttura risposta Google
type googleUserInfo struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Picture   string `json:"picture"`
	GivenName string `json:"given_name"`
}

func getGoogleUserInfo(accessToken string) (*googleUserInfo, error) {
	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + accessToken)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var info googleUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func (h *OAuthHandler) findOrCreateGoogleUser(ctx context.Context, g *googleUserInfo) (string, error) {
	var userID string

	// Cerca per google_id
	err := h.pg.Pool.QueryRow(ctx,
		`SELECT id FROM users WHERE google_id = $1`, g.ID,
	).Scan(&userID)

	if err == nil {
		// Aggiorna last_seen
		h.pg.Pool.Exec(ctx, `UPDATE users SET last_seen = NOW() WHERE id = $1`, userID)
		return userID, nil
	}

	// Cerca per email (utente già registrato con email)
	err = h.pg.Pool.QueryRow(ctx,
		`SELECT id FROM users WHERE email = $1`, g.Email,
	).Scan(&userID)

	if err == nil {
		// Collega il google_id all'account esistente
		h.pg.Pool.Exec(ctx,
			`UPDATE users SET google_id = $1, avatar_url = $2, last_seen = NOW() WHERE id = $3`,
			g.ID, g.Picture, userID,
		)
		return userID, nil
	}

	// Crea nuovo utente
	username := sanitizeUsername(g.GivenName)
	err = h.pg.Pool.QueryRow(ctx,
		`INSERT INTO users (username, email, google_id, avatar_url)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		username, g.Email, g.ID, g.Picture,
	).Scan(&userID)

	if err != nil {
		return "", fmt.Errorf("creazione utente fallita: %w", err)
	}

	return userID, nil
}

// Genera username univoco da nome Google
func sanitizeUsername(name string) string {
	if len(name) == 0 {
		return "player"
	}
	// Rimuove spazi e caratteri speciali
	result := ""
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			result += string(c)
		}
	}
	if len(result) == 0 {
		return "player"
	}
	if len(result) > 20 {
		return result[:20]
	}
	return result
}

func getEnvOAuth(key string) string {
	return os.Getenv(key)
}

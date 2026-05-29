package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"chess-clone/backend/internal/db"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	pg     *db.Postgres
	cfg    *AdminConfig
	mailer *Mailer
}

func NewAuthHandler(pg *db.Postgres, cfg *AdminConfig, mailer *Mailer) *AuthHandler {
	return &AuthHandler{pg: pg, cfg: cfg, mailer: mailer}
}

var jwtSecret = []byte("change_me_in_production")

const (
	sessionDuration = 30 * 24 * time.Hour          // durata JWT
	sessionMaxAge   = 30 * 24 * 60 * 60            // MaxAge cookie in secondi
)

// POST /api/auth/register
// Crea l'account con is_verified=false, invia email di verifica.
// Non imposta il cookie — l'utente deve prima verificare l'email.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "Richiesta non valida")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SERVER_ERROR", "Errore interno")
		return
	}

	var userID string
	err = h.pg.Pool.QueryRow(r.Context(),
		`INSERT INTO users (username, email, password_hash, is_verified)
		 VALUES ($1, $2, $3, false) RETURNING id`,
		body.Username, body.Email, string(hash),
	).Scan(&userID)
	if err != nil {
		writeError(w, http.StatusConflict, "USER_EXISTS", "Username o email già in uso")
		return
	}

	// Genera e salva il token di verifica
	token, err := GenerateToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SERVER_ERROR", "Errore interno")
		return
	}
	if _, err := h.pg.Pool.Exec(r.Context(),
		`INSERT INTO email_verifications (token, user_id) VALUES ($1, $2)`,
		token, userID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "SERVER_ERROR", "Errore interno")
		return
	}

	// Invio email — non blocca il response in caso di errore SMTP
	if err := h.mailer.SendVerificationEmail(body.Email, token); err != nil {
		log.Printf("⚠️  Invio email verifica fallito (%s): %v", body.Email, err)
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"status": "email_sent",
		"email":  body.Email,
	})
}

// POST /api/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "Richiesta non valida")
		return
	}

	var userID, passwordHash string
	var isBanned, isVerified bool
	err := h.pg.Pool.QueryRow(r.Context(),
		`SELECT id, password_hash,
		        COALESCE(is_banned,   false) AS is_banned,
		        COALESCE(is_verified, true)  AS is_verified
		 FROM users WHERE email = $1`,
		body.Email,
	).Scan(&userID, &passwordHash, &isBanned, &isVerified)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Email o password errati")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(body.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Email o password errati")
		return
	}

	if isBanned {
		writeError(w, http.StatusForbidden, "BANNED", "Account sospeso")
		return
	}

	if !isVerified {
		writeError(w, http.StatusForbidden, "EMAIL_NOT_VERIFIED", "Verifica la tua email prima di accedere")
		return
	}

	token, err := generateJWT(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SERVER_ERROR", "Errore interno")
		return
	}

	setAuthCookie(w, token)
	writeJSON(w, http.StatusOK, map[string]string{"user_id": userID})
}

// POST /api/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		MaxAge:   -1,
		HttpOnly: true,
		Path:     "/",
	})
	writeJSON(w, http.StatusOK, map[string]string{"message": "logout effettuato"})
}

// GET /api/auth/me
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, err := getUserIDFromCookie(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Non autenticato")
		return
	}

	var user struct {
		ID        string `json:"id"`
		Username  string `json:"username"`
		Email     string `json:"email"`
		EloRapid  int    `json:"elo_rapid"`
		EloBlitz  int    `json:"elo_blitz"`
		EloBullet int    `json:"elo_bullet"`
		IsAdmin   bool   `json:"is_admin"`
	}
	err = h.pg.Pool.QueryRow(r.Context(),
		`SELECT id, username, email, elo_rapid, elo_blitz, elo_bullet
		 FROM users WHERE id = $1`, userID,
	).Scan(&user.ID, &user.Username, &user.Email,
		&user.EloRapid, &user.EloBlitz, &user.EloBullet)
	if err != nil {
		writeError(w, http.StatusNotFound, "USER_NOT_FOUND", "Utente non trovato")
		return
	}

	user.IsAdmin = h.cfg.IsAdmin(user.Email)

	// Sliding expiration: rinnova il cookie ad ogni richiesta autenticata.
	// Finché l'utente visita il sito almeno una volta ogni 30 giorni
	// il cookie rimane sempre fresco e non viene mai richiesto il re-login.
	if freshToken, err := generateJWT(user.ID); err == nil {
		setAuthCookie(w, freshToken)
	}

	writeJSON(w, http.StatusOK, user)
}

// POST /api/auth/verify-email
// Verifica il token ricevuto via email. Imposta is_verified=true e restituisce il cookie JWT.
func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Token == "" {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "Token richiesto")
		return
	}

	var userID string
	err := h.pg.Pool.QueryRow(r.Context(),
		`SELECT user_id FROM email_verifications
		 WHERE token = $1 AND expires_at > NOW()`,
		body.Token,
	).Scan(&userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_TOKEN", "Link non valido o scaduto")
		return
	}

	// Segna l'utente come verificato
	if _, err := h.pg.Pool.Exec(r.Context(),
		`UPDATE users SET is_verified = true WHERE id = $1`, userID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "SERVER_ERROR", "Errore interno")
		return
	}

	// Rimuovi il token usato (uno shot)
	h.pg.Pool.Exec(r.Context(), `DELETE FROM email_verifications WHERE token = $1`, body.Token)

	// Rilascia il cookie JWT — l'utente è loggato
	jwtToken, err := generateJWT(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SERVER_ERROR", "Errore interno")
		return
	}
	setAuthCookie(w, jwtToken)

	writeJSON(w, http.StatusOK, map[string]string{"status": "verified"})
}

// POST /api/auth/resend-verification
// Reinvia l'email di verifica. Risponde sempre 200 per non rivelare l'esistenza dell'account.
func (h *AuthHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "Email richiesta")
		return
	}

	var userID string
	var isVerified bool
	err := h.pg.Pool.QueryRow(r.Context(),
		`SELECT id, COALESCE(is_verified, true) FROM users WHERE email = $1`,
		body.Email,
	).Scan(&userID, &isVerified)

	// Non rivelare se l'email esiste
	if err != nil || isVerified {
		writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
		return
	}

	// Elimina token precedenti e ne crea uno nuovo
	h.pg.Pool.Exec(r.Context(), `DELETE FROM email_verifications WHERE user_id = $1`, userID)

	token, err := GenerateToken()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
		return
	}
	h.pg.Pool.Exec(r.Context(),
		`INSERT INTO email_verifications (token, user_id) VALUES ($1, $2)`, token, userID,
	)

	if err := h.mailer.SendVerificationEmail(body.Email, token); err != nil {
		log.Printf("⚠️  Rinvio email verifica fallito (%s): %v", body.Email, err)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

// POST /api/auth/dev-login  (registrato solo se DEV_MODE=true)
func (h *AuthHandler) DevLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Username == "" {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "username richiesto")
		return
	}

	var userID string
	err := h.pg.Pool.QueryRow(r.Context(),
		`SELECT id FROM users WHERE username = $1`, body.Username,
	).Scan(&userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "USER_NOT_FOUND", "Utente non trovato")
		return
	}

	token, err := generateJWT(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SERVER_ERROR", "Errore interno")
		return
	}

	setAuthCookie(w, token)
	writeJSON(w, http.StatusOK, map[string]string{"user_id": userID})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func generateJWT(userID string) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(sessionDuration).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(jwtSecret)
}

func getUserIDFromCookie(r *http.Request) (string, error) {
	cookie, err := r.Cookie("auth_token")
	if err != nil {
		return "", err
	}
	token, err := jwt.Parse(cookie.Value, func(t *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return "", err
	}
	claims := token.Claims.(jwt.MapClaims)
	return claims["sub"].(string), nil
}

func setAuthCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		HttpOnly: true,
		Secure:   false, // true in produzione
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   sessionMaxAge,
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"success": false,
		"error":   map[string]string{"code": code, "message": message},
	})
}

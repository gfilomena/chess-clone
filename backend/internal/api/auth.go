package api

import (
	"encoding/json"
	"net/http"
	"time"

	"chess-clone/backend/internal/db"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	pg  *db.Postgres
	rdb *db.Redis
}

func NewAuthHandler(pg *db.Postgres, rdb *db.Redis) *AuthHandler {
	return &AuthHandler{pg: pg, rdb: rdb}
}

var jwtSecret = []byte("change_me_in_production")

// POST /api/auth/register
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
		`INSERT INTO users (username, email, password_hash)
		 VALUES ($1, $2, $3) RETURNING id`,
		body.Username, body.Email, string(hash),
	).Scan(&userID)
	if err != nil {
		writeError(w, http.StatusConflict, "USER_EXISTS", "Username o email già in uso")
		return
	}

	token, err := generateJWT(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SERVER_ERROR", "Errore interno")
		return
	}

	setAuthCookie(w, token)
	writeJSON(w, http.StatusCreated, map[string]string{"user_id": userID})
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
	err := h.pg.Pool.QueryRow(r.Context(),
		`SELECT id, password_hash FROM users WHERE email = $1`,
		body.Email,
	).Scan(&userID, &passwordHash)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Email o password errati")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(body.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Email o password errati")
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

	writeJSON(w, http.StatusOK, user)
}

// POST /api/auth/dev-login  (registrato solo se DEV_MODE=true)
// Accede con il solo username, senza password — esclusivamente per sviluppo locale.
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

// Helpers
func generateJWT(userID string) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(7 * 24 * time.Hour).Unix(),
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
		MaxAge:   7 * 24 * 60 * 60,
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

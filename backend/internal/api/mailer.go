package api

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"os"
	"strings"
	"time"
)

// Mailer gestisce l'invio di email transazionali.
// Se SMTP_HOST non è configurato si comporta in modalità dev: stampa il link a stdout.
type Mailer struct {
	host     string
	port     string
	user     string
	password string
	from     string
	// URL base del frontend — usato per costruire i link nelle email
	frontendURL string
	devMode     bool
}

func NewMailer() *Mailer {
	host := os.Getenv("SMTP_HOST")
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:5174"
	}
	return &Mailer{
		host:        host,
		port:        envOrDefault("SMTP_PORT", "587"),
		user:        os.Getenv("SMTP_USER"),
		password:    os.Getenv("SMTP_PASSWORD"),
		from:        envOrDefault("SMTP_FROM", "noreply@chess.app"),
		frontendURL: frontendURL,
		devMode:     host == "",
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// GenerateToken genera un token esadecimale sicuro da 32 byte (64 caratteri).
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// SendVerificationEmail invia (o logga in dev) la email di verifica account.
func (m *Mailer) SendVerificationEmail(toEmail, token string) error {
	verifyURL := fmt.Sprintf("%s/verify-email?token=%s", m.frontendURL, token)

	subject := "Verifica il tuo account Chess"
	body := fmt.Sprintf(
		"Ciao!\r\n\r\n"+
			"Clicca il link seguente per verificare il tuo account:\r\n\r\n"+
			"%s\r\n\r\n"+
			"Il link scade tra 24 ore.\r\n\r\n"+
			"Se non hai creato un account, ignora questa email.\r\n\r\n"+
			"— Chess",
		verifyURL,
	)

	if m.devMode {
		log.Printf("📧  [DEV] Verifica email → %s\n    URL: %s", toEmail, verifyURL)
		return nil
	}

	return m.sendSMTP(toEmail, subject, body)
}

func (m *Mailer) sendSMTP(to, subject, body string) error {
	msg := strings.Join([]string{
		fmt.Sprintf("From: Chess <%s>", m.from),
		fmt.Sprintf("To: %s", to),
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		`Content-Type: text/plain; charset="UTF-8"`,
		"",
		body,
	}, "\r\n")

	addr := net.JoinHostPort(m.host, m.port)

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("connessione SMTP fallita (%s): %w", addr, err)
	}

	client, err := smtp.NewClient(conn, m.host)
	if err != nil {
		return fmt.Errorf("client SMTP fallito: %w", err)
	}
	defer client.Close()

	if err := client.StartTLS(&tls.Config{ServerName: m.host}); err != nil {
		return fmt.Errorf("STARTTLS fallito: %w", err)
	}

	auth := smtp.PlainAuth("", m.user, m.password, m.host)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("auth SMTP fallita: %w", err)
	}

	if err := client.Mail(m.from); err != nil {
		return fmt.Errorf("MAIL FROM fallito: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT TO fallito: %w", err)
	}

	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA fallito: %w", err)
	}
	defer wc.Close()

	_, err = fmt.Fprint(wc, msg)
	return err
}

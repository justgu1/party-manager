// Package users handles registration, login, password reset and admin seeding.
package users

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/guilherme/help-party/internal/auth"
	"github.com/guilherme/help-party/internal/config"
	"github.com/guilherme/help-party/internal/httpx"
	"github.com/guilherme/help-party/internal/mailer"
)

type Handler struct {
	pool *pgxpool.Pool
	auth *auth.Service
	mail *mailer.Mailer
	cfg  config.Config
}

func New(pool *pgxpool.Pool, authSvc *auth.Service, mail *mailer.Mailer, cfg config.Config) *Handler {
	return &Handler{pool: pool, auth: authSvc, mail: mail, cfg: cfg}
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type authResponse struct {
	Token string   `json:"token"`
	User  userView `json:"user"`
}

type userView struct {
	ID      uuid.UUID `json:"id"`
	Email   string    `json:"email"`
	Name    string    `json:"name"`
	IsAdmin bool      `json:"is_admin"`
}

// Register creates a new (non-admin) user and returns a JWT.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !httpx.Decode(w, r, &in) {
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Name = strings.TrimSpace(in.Name)
	if in.Email == "" || in.Password == "" {
		httpx.Error(w, http.StatusBadRequest, "email e senha são obrigatórios")
		return
	}
	if in.Name == "" {
		in.Name = in.Email
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao processar senha")
		return
	}

	var u auth.User
	err = h.pool.QueryRow(r.Context(),
		`INSERT INTO users (email, password_hash, name)
		 VALUES ($1, $2, $3) RETURNING id, email, name, is_admin`,
		in.Email, hash, in.Name,
	).Scan(&u.ID, &u.Email, &u.Name, &u.IsAdmin)
	if err != nil {
		if strings.Contains(err.Error(), "users_email_key") {
			httpx.Error(w, http.StatusConflict, "email já cadastrado")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao criar usuário")
		return
	}
	h.respondToken(w, u)
}

// Login validates credentials and returns a JWT.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var in credentials
	if !httpx.Decode(w, r, &in) {
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))

	var (
		u    auth.User
		hash string
	)
	err := h.pool.QueryRow(r.Context(),
		`SELECT id, email, name, is_admin, password_hash FROM users WHERE email = $1`,
		in.Email,
	).Scan(&u.ID, &u.Email, &u.Name, &u.IsAdmin, &hash)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !auth.CheckPassword(hash, in.Password)) {
		httpx.Error(w, http.StatusUnauthorized, "credenciais inválidas")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro no login")
		return
	}
	h.respondToken(w, u)
}

// Me returns the currently authenticated user.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "não autenticado")
		return
	}
	httpx.JSON(w, http.StatusOK, userView{ID: u.ID, Email: u.Email, Name: u.Name, IsAdmin: u.IsAdmin})
}

type resetRequestReq struct {
	Email string `json:"email"`
}

// ResetRequest issues a password-reset token and emails a link. It always
// responds 200 to avoid leaking which emails exist.
func (h *Handler) ResetRequest(w http.ResponseWriter, r *http.Request) {
	var in resetRequestReq
	if !httpx.Decode(w, r, &in) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))

	var userID uuid.UUID
	err := h.pool.QueryRow(r.Context(), `SELECT id FROM users WHERE email=$1`, email).Scan(&userID)
	if err == nil {
		if token, e := h.createResetToken(r.Context(), userID); e == nil {
			h.sendResetEmail(email, token)
		}
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type resetReq struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// Reset consumes a token and sets a new password.
func (h *Handler) Reset(w http.ResponseWriter, r *http.Request) {
	var in resetReq
	if !httpx.Decode(w, r, &in) {
		return
	}
	if len(in.Password) < 6 {
		httpx.Error(w, http.StatusBadRequest, "senha deve ter ao menos 6 caracteres")
		return
	}
	token, err := uuid.Parse(strings.TrimSpace(in.Token))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "token inválido")
		return
	}

	var userID uuid.UUID
	err = h.pool.QueryRow(r.Context(),
		`SELECT user_id FROM password_resets
		 WHERE token=$1 AND used=FALSE AND expires_at > now()`, token,
	).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusBadRequest, "token inválido ou expirado")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao validar token")
		return
	}

	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao processar senha")
		return
	}
	if _, err := h.pool.Exec(r.Context(),
		`UPDATE users SET password_hash=$2 WHERE id=$1`, userID, hash); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao atualizar senha")
		return
	}
	_, _ = h.pool.Exec(r.Context(), `UPDATE password_resets SET used=TRUE WHERE token=$1`, token)
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Seed ensures the configured admin emails exist as admin users. New admins get
// a random password and a "set your password" email (logged if SMTP is off).
func (h *Handler) Seed(ctx context.Context) error {
	for _, email := range h.cfg.AdminEmails {
		var (
			id      uuid.UUID
			isAdmin bool
		)
		err := h.pool.QueryRow(ctx, `SELECT id, is_admin FROM users WHERE email=$1`, email).
			Scan(&id, &isAdmin)

		switch {
		case errors.Is(err, pgx.ErrNoRows):
			// Create a new admin with a random password + reset email.
			hash, _ := auth.HashPassword(randomSecret())
			if e := h.pool.QueryRow(ctx,
				`INSERT INTO users (email, password_hash, name, is_admin)
				 VALUES ($1, $2, $3, TRUE) RETURNING id`,
				email, hash, email,
			).Scan(&id); e != nil {
				log.Printf("seed admin %s: %v", email, e)
				continue
			}
			if token, e := h.createResetToken(ctx, id); e == nil {
				h.sendResetEmail(email, token)
			}
			log.Printf("seeded admin %s", email)
		case err != nil:
			log.Printf("seed check %s: %v", email, err)
		case !isAdmin:
			// Promote an existing user to admin.
			_, _ = h.pool.Exec(ctx, `UPDATE users SET is_admin=TRUE WHERE id=$1`, id)
			log.Printf("promoted %s to admin", email)
		}
	}
	return nil
}

func (h *Handler) createResetToken(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	var token uuid.UUID
	err := h.pool.QueryRow(ctx,
		`INSERT INTO password_resets (user_id, expires_at)
		 VALUES ($1, $2) RETURNING token`,
		userID, time.Now().Add(72*time.Hour),
	).Scan(&token)
	return token, err
}

func (h *Handler) sendResetEmail(email string, token uuid.UUID) {
	link := strings.TrimRight(h.cfg.AppBaseURL, "/") + "/#/reset?token=" + token.String()
	body := "Olá!\n\nVocê foi adicionado(a) como administrador(a) do help-party.\n" +
		"Defina sua senha neste link (válido por 72h):\n\n" + link + "\n\nAté a festa! 🎉"
	if err := h.mail.Send(email, "help-party — defina sua senha", body); err != nil {
		log.Printf("reset email to %s failed: %v", email, err)
	}
}

func (h *Handler) respondToken(w http.ResponseWriter, u auth.User) {
	token, err := h.auth.Issue(u)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao emitir token")
		return
	}
	httpx.JSON(w, http.StatusOK, authResponse{
		Token: token,
		User:  userView{ID: u.ID, Email: u.Email, Name: u.Name, IsAdmin: u.IsAdmin},
	})
}

func randomSecret() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

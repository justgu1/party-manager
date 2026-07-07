package users

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/guilherme/help-party/internal/auth"
	"github.com/guilherme/help-party/internal/httpx"
)

// AdminRoutes mounts admin-only user management (behind RequireAdmin).
func (h *Handler) AdminRoutes(r chi.Router, admin func(http.Handler) http.Handler) {
	r.With(admin).Get("/users", h.ListUsers)
	r.With(admin).Post("/users", h.CreateUser)
	r.With(admin).Put("/users/{id}", h.UpdateUser)
	r.With(admin).Delete("/users/{id}", h.DeleteUser)
	r.With(admin).Post("/users/{id}/reset", h.SendReset)
}

type adminUserInput struct {
	Email   string `json:"email"`
	Name    string `json:"name"`
	IsAdmin bool   `json:"is_admin"`
}

// ListUsers returns all users.
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, email, name, is_admin FROM users ORDER BY created_at ASC`)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar usuários")
		return
	}
	defer rows.Close()
	out := []userView{}
	for rows.Next() {
		var u userView
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.IsAdmin); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "erro ao ler usuário")
			return
		}
		out = append(out, u)
	}
	httpx.JSON(w, http.StatusOK, out)
}

// CreateUser provisions a user with a random password and emails a "set your
// password" link (logged if SMTP is off).
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var in adminUserInput
	if !httpx.Decode(w, r, &in) {
		return
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Name = strings.TrimSpace(in.Name)
	if in.Email == "" {
		httpx.Error(w, http.StatusBadRequest, "email é obrigatório")
		return
	}
	if in.Name == "" {
		in.Name = in.Email
	}
	hash, err := auth.HashPassword(randomSecret())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao processar senha")
		return
	}
	var u userView
	err = h.pool.QueryRow(r.Context(),
		`INSERT INTO users (email, password_hash, name, is_admin)
		 VALUES ($1,$2,$3,$4) RETURNING id, email, name, is_admin`,
		in.Email, hash, in.Name, in.IsAdmin,
	).Scan(&u.ID, &u.Email, &u.Name, &u.IsAdmin)
	if err != nil {
		if strings.Contains(err.Error(), "users_email_key") {
			httpx.Error(w, http.StatusConflict, "email já cadastrado")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao criar usuário")
		return
	}
	if token, e := h.createResetToken(r.Context(), u.ID); e == nil {
		h.sendResetEmail(u.Email, token)
	}
	httpx.JSON(w, http.StatusCreated, u)
}

// UpdateUser edits a user's name and admin flag.
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseID(w, r)
	if !ok {
		return
	}
	var in adminUserInput
	if !httpx.Decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	var u userView
	err := h.pool.QueryRow(r.Context(),
		`UPDATE users SET name=COALESCE(NULLIF($2,''), name), is_admin=$3 WHERE id=$1
		 RETURNING id, email, name, is_admin`,
		id, in.Name, in.IsAdmin,
	).Scan(&u.ID, &u.Email, &u.Name, &u.IsAdmin)
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, http.StatusNotFound, "usuário não encontrado")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao atualizar")
		return
	}
	httpx.JSON(w, http.StatusOK, u)
}

// DeleteUser removes a user (an admin cannot delete themselves).
func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseID(w, r)
	if !ok {
		return
	}
	if me, _ := auth.FromContext(r.Context()); me.ID == id {
		httpx.Error(w, http.StatusBadRequest, "você não pode excluir a si mesmo")
		return
	}
	if _, err := h.pool.Exec(r.Context(), `DELETE FROM users WHERE id=$1`, id); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao excluir")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// SendReset issues a fresh password-reset link for a user.
func (h *Handler) SendReset(w http.ResponseWriter, r *http.Request) {
	id, ok := h.parseID(w, r)
	if !ok {
		return
	}
	var email string
	if err := h.pool.QueryRow(r.Context(), `SELECT email FROM users WHERE id=$1`, id).Scan(&email); err != nil {
		httpx.Error(w, http.StatusNotFound, "usuário não encontrado")
		return
	}
	if token, e := h.createResetToken(r.Context(), id); e == nil {
		h.sendResetEmail(email, token)
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "id inválido")
		return uuid.Nil, false
	}
	return id, true
}

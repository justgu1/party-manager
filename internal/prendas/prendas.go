// Package prendas is a small CRUD for the catalog of (symbolic) prendas that
// unlock songs in the jukebox.
package prendas

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/guilherme/help-party/internal/httpx"
)

type Handler struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Handler {
	return &Handler{pool: pool}
}

// Routes mounts the prenda endpoints. Listing is open to all authenticated
// users (the jukebox needs it); create/update/delete are admin-only.
func (h *Handler) Routes(r chi.Router, admin func(http.Handler) http.Handler) {
	r.Get("/prendas", h.List)
	r.With(admin).Post("/prendas", h.Create)
	r.With(admin).Put("/prendas/{id}", h.Update)
	r.With(admin).Delete("/prendas/{id}", h.Delete)
}

type prenda struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type prendaInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(),
		`SELECT id, title, description, created_at FROM prendas ORDER BY created_at DESC`)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar prendas")
		return
	}
	defer rows.Close()
	out := []prenda{}
	for rows.Next() {
		var p prenda
		if err := rows.Scan(&p.ID, &p.Title, &p.Description, &p.CreatedAt); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "erro ao ler prenda")
			return
		}
		out = append(out, p)
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in prendaInput
	if !httpx.Decode(w, r, &in) {
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		httpx.Error(w, http.StatusBadRequest, "título é obrigatório")
		return
	}
	var p prenda
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO prendas (title, description) VALUES ($1, $2)
		 RETURNING id, title, description, created_at`,
		in.Title, in.Description,
	).Scan(&p.ID, &p.Title, &p.Description, &p.CreatedAt)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao criar prenda")
		return
	}
	httpx.JSON(w, http.StatusCreated, p)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "id inválido")
		return
	}
	var in prendaInput
	if !httpx.Decode(w, r, &in) {
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		httpx.Error(w, http.StatusBadRequest, "título é obrigatório")
		return
	}
	var p prenda
	err = h.pool.QueryRow(r.Context(),
		`UPDATE prendas SET title=$2, description=$3 WHERE id=$1
		 RETURNING id, title, description, created_at`,
		id, in.Title, in.Description,
	).Scan(&p.ID, &p.Title, &p.Description, &p.CreatedAt)
	if err == pgx.ErrNoRows {
		httpx.Error(w, http.StatusNotFound, "prenda não encontrada")
		return
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao atualizar prenda")
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "id inválido")
		return
	}
	if _, err := h.pool.Exec(r.Context(), `DELETE FROM prendas WHERE id=$1`, id); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao remover prenda")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

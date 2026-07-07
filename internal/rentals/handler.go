package rentals

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/guilherme/help-party/internal/auth"
	"github.com/guilherme/help-party/internal/httpx"
)

type Handler struct {
	pool  *pgxpool.Pool
	river *river.Client[pgx.Tx]
}

func New(pool *pgxpool.Pool, riverClient *river.Client[pgx.Tx]) *Handler {
	return &Handler{pool: pool, river: riverClient}
}

// Routes mounts the rental endpoints. Listing/voting are open to all
// authenticated users; adding places, editing details and running the
// tournament are admin-only.
func (h *Handler) Routes(r chi.Router, admin func(http.Handler) http.Handler) {
	r.Get("/rentals", h.List)
	r.Get("/rentals/tournament", h.Tournament)
	r.Get("/rentals/{id}", h.Get)
	r.Post("/rentals/{id}/vote", h.Vote)
	r.Delete("/rentals/{id}/vote", h.Unvote)

	r.With(admin).Post("/rentals", h.Create)
	r.With(admin).Put("/rentals/{id}", h.Update)
	r.With(admin).Delete("/rentals/{id}", h.Delete)
}

type rentalView struct {
	ID           uuid.UUID      `json:"id"`
	URL          string         `json:"url"`
	Source       string         `json:"source"`
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	Price        string         `json:"price"`
	Rating       string         `json:"rating"`
	ReviewsCount int            `json:"reviews_count"`
	ImageURL     string         `json:"image_url"`
	Status       string         `json:"status"`
	Error        string         `json:"error"`
	Notes        string         `json:"notes"`
	Score        int            `json:"score"`
	Upvotes      int            `json:"upvotes"`
	Downvotes    int            `json:"downvotes"`
	MyVote       int            `json:"my_vote"`
	Rank         int            `json:"rank"`
	Advancing    bool           `json:"advancing"`
	Eliminated   bool           `json:"eliminated"`
	Availability []availability `json:"availability"`
	CreatedAt    time.Time      `json:"created_at"`
}

type availability struct {
	Label string     `json:"label"`
	From  *time.Time `json:"from,omitempty"`
	To    *time.Time `json:"to,omitempty"`
}

type createReq struct {
	URL string `json:"url"`
}

// Create inserts a rental in the `pending` state and enqueues a scrape job.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromContext(r.Context())
	var in createReq
	if !httpx.Decode(w, r, &in) {
		return
	}
	in.URL = strings.TrimSpace(in.URL)
	if !strings.HasPrefix(in.URL, "http") {
		httpx.Error(w, http.StatusBadRequest, "informe uma URL válida")
		return
	}

	var id uuid.UUID
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO rentals (url, submitted_by, status) VALUES ($1, $2, 'pending') RETURNING id`,
		in.URL, u.ID,
	).Scan(&id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "não foi possível salvar o link")
		return
	}

	if _, err := h.river.Insert(r.Context(), ScrapeRentalArgs{RentalID: id.String(), URL: in.URL}, nil); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "não foi possível enfileirar o scraping")
		return
	}
	h.writeOne(w, r, id)
}

type updateReq struct {
	Notes string `json:"notes"`
}

// Update lets an admin add manual notes/details to a place.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var in updateReq
	if !httpx.Decode(w, r, &in) {
		return
	}
	if _, err := h.pool.Exec(r.Context(),
		`UPDATE rentals SET notes=$2, updated_at=now() WHERE id=$1`, id, in.Notes); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao salvar detalhes")
		return
	}
	h.writeOne(w, r, id)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if _, err := h.pool.Exec(r.Context(), `DELETE FROM rentals WHERE id=$1`, id); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao remover")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// List returns all rentals ranked by net score (desc), annotated with the
// tournament advancing/eliminated status.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromContext(r.Context())
	rows, err := h.pool.Query(r.Context(), listQuery, u.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar")
		return
	}
	defer rows.Close()

	out := []rentalView{}
	for rows.Next() {
		v, err := scanRental(rows)
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "erro ao ler linha")
			return
		}
		out = append(out, v)
	}
	rows.Close()

	for i := range out {
		out[i].Rank = i + 1
		_ = h.loadAvailability(r, &out[i])
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	h.writeOne(w, r, id)
}

type voteReq struct {
	Value int `json:"value"` // +1 up, -1 down
}

// Vote records the current user's up (+1) or down (-1) vote (upsert).
func (h *Handler) Vote(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromContext(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var in voteReq
	if !httpx.Decode(w, r, &in) {
		return
	}
	value := 1
	if in.Value < 0 {
		value = -1
	}
	_, err := h.pool.Exec(r.Context(),
		`INSERT INTO rental_votes (rental_id, user_id, value) VALUES ($1, $2, $3)
		 ON CONFLICT (rental_id, user_id) DO UPDATE SET value = EXCLUDED.value`,
		id, u.ID, value)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao votar")
		return
	}
	h.writeOne(w, r, id)
}

// Unvote removes the current user's vote entirely.
func (h *Handler) Unvote(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromContext(r.Context())
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	_, err := h.pool.Exec(r.Context(),
		`DELETE FROM rental_votes WHERE rental_id=$1 AND user_id=$2`, id, u.ID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao remover voto")
		return
	}
	h.writeOne(w, r, id)
}

const listSelect = `
	SELECT r.id, r.url, r.source, r.title, r.description, r.price, r.rating,
	       r.reviews_count, r.image_url, r.status, r.error, r.notes,
	       COALESCE(SUM(v.value), 0)::int AS score,
	       COALESCE(SUM(CASE WHEN v.value > 0 THEN 1 ELSE 0 END), 0)::int AS upvotes,
	       COALESCE(SUM(CASE WHEN v.value < 0 THEN 1 ELSE 0 END), 0)::int AS downvotes,
	       COALESCE(SUM(CASE WHEN v.user_id = $1 THEN v.value ELSE 0 END), 0)::int AS my_vote,
	       r.created_at
	FROM rentals r
	LEFT JOIN rental_votes v ON v.rental_id = r.id`

const listQuery = listSelect + `
	GROUP BY r.id
	ORDER BY score DESC, r.created_at DESC`

const oneQuery = listSelect + `
	WHERE r.id = $2
	GROUP BY r.id`

func scanRental(rows pgx.Rows) (rentalView, error) {
	var v rentalView
	err := rows.Scan(&v.ID, &v.URL, &v.Source, &v.Title, &v.Description, &v.Price,
		&v.Rating, &v.ReviewsCount, &v.ImageURL, &v.Status, &v.Error, &v.Notes,
		&v.Score, &v.Upvotes, &v.Downvotes, &v.MyVote, &v.CreatedAt)
	v.Availability = []availability{}
	return v, err
}

func (h *Handler) loadAvailability(r *http.Request, v *rentalView) error {
	rows, err := h.pool.Query(r.Context(),
		`SELECT label, date_from, date_to FROM rental_availability WHERE rental_id=$1 ORDER BY date_from`,
		v.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var a availability
		if err := rows.Scan(&a.Label, &a.From, &a.To); err != nil {
			return err
		}
		v.Availability = append(v.Availability, a)
	}
	return nil
}

func (h *Handler) writeOne(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	u, _ := auth.FromContext(r.Context())
	rows, err := h.pool.Query(r.Context(), oneQuery, u.ID, id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	defer rows.Close()
	if !rows.Next() {
		httpx.Error(w, http.StatusNotFound, "não encontrado")
		return
	}
	v, err := scanRental(rows)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao ler")
		return
	}
	rows.Close()
	_ = h.loadAvailability(r, &v)
	httpx.JSON(w, http.StatusOK, v)
}

func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "id inválido")
		return uuid.Nil, false
	}
	return id, true
}

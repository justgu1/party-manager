// Package music implements the YouTube jukebox: songs are queued with a chosen
// prenda and only become playable once that prenda is marked as done.
package music

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/guilherme/help-party/internal/auth"
	"github.com/guilherme/help-party/internal/config"
	"github.com/guilherme/help-party/internal/httpx"
)

type Handler struct {
	pool *pgxpool.Pool
	cfg  config.Config
}

func New(pool *pgxpool.Pool, cfg config.Config) *Handler {
	return &Handler{pool: pool, cfg: cfg}
}

// Routes mounts the song endpoints. All are available to authenticated users
// (the admin middleware is accepted for signature uniformity).
func (h *Handler) Routes(r chi.Router, _ func(http.Handler) http.Handler) {
	r.Get("/songs", h.List)
	r.Get("/songs/search", h.Search)
	r.Post("/songs", h.Create)
	r.Post("/songs/{id}/prenda-done", h.MarkPrendaDone)
	r.Post("/songs/{id}/played", h.MarkPlayed)
	r.Post("/songs/{id}/requeue", h.Requeue)
	r.Delete("/songs/{id}", h.Delete)
}

// Search proxies the YouTube Data API search so users can find songs by name.
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		httpx.JSON(w, http.StatusOK, []SearchResult{})
		return
	}
	results, err := Search(r.Context(), httpClient, h.cfg.YouTubeAPIKey, q)
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, results)
}

type song struct {
	ID           uuid.UUID  `json:"id"`
	YouTubeID    string     `json:"youtube_id"`
	URL          string     `json:"url"`
	Title        string     `json:"title"`
	ThumbnailURL string     `json:"thumbnail_url"`
	Author       string     `json:"author"`
	PrendaID     *uuid.UUID `json:"prenda_id"`
	PrendaTitle  string     `json:"prenda_title"`
	PrendaDone   bool       `json:"prenda_done"`
	Status       string     `json:"status"`
	RequestedBy  string     `json:"requested_by"`
	CreatedAt    time.Time  `json:"created_at"`
}

type createReq struct {
	URL      string     `json:"url"`
	PrendaID *uuid.UUID `json:"prenda_id"`
}

const selectSongs = `
	SELECT s.id, s.youtube_id, s.url, s.title, s.thumbnail_url, s.author,
	       s.prenda_id, COALESCE(p.title, ''), s.prenda_done, s.status,
	       COALESCE(u.name, ''), s.created_at
	FROM songs s
	LEFT JOIN prendas p ON p.id = s.prenda_id
	LEFT JOIN users u ON u.id = s.requested_by`

// List returns the queue ordered by insertion (position).
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(), selectSongs+` ORDER BY s.position ASC`)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar músicas")
		return
	}
	defer rows.Close()
	out := []song{}
	for rows.Next() {
		s, err := scanSong(rows)
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "erro ao ler música")
			return
		}
		out = append(out, s)
	}
	httpx.JSON(w, http.StatusOK, out)
}

// Create resolves the YouTube URL to metadata (oEmbed) and enqueues the song.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	u, _ := auth.FromContext(r.Context())
	var in createReq
	if !httpx.Decode(w, r, &in) {
		return
	}
	if in.PrendaID == nil {
		httpx.Error(w, http.StatusBadRequest, "escolha uma prenda para a música")
		return
	}
	videoID, ok := ExtractVideoID(in.URL)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "link do YouTube inválido")
		return
	}
	canonical := canonicalURL(videoID)

	// oEmbed is best-effort: if it fails we still queue the song with the id.
	oe, err := FetchOEmbed(r.Context(), httpClient, canonical)
	if err != nil {
		oe = OEmbed{Title: canonical}
	}

	var id uuid.UUID
	err = h.pool.QueryRow(r.Context(),
		`INSERT INTO songs (youtube_id, url, title, thumbnail_url, author, requested_by, prenda_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		videoID, canonical, oe.Title, oe.ThumbnailURL, oe.AuthorName, u.ID, in.PrendaID,
	).Scan(&id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao adicionar música")
		return
	}
	h.writeOne(w, r, id)
}

// MarkPrendaDone unlocks a song by flagging its prenda as fulfilled.
func (h *Handler) MarkPrendaDone(w http.ResponseWriter, r *http.Request) {
	h.setBool(w, r, "prenda_done", true)
}

// MarkPlayed marks a song as already played.
func (h *Handler) MarkPlayed(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if _, err := h.pool.Exec(r.Context(),
		`UPDATE songs SET status='played' WHERE id=$1`, id); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao atualizar")
		return
	}
	h.writeOne(w, r, id)
}

// Requeue puts a played song back at the end of the queue. Its prenda stays
// fulfilled, so it is ready to play again.
func (h *Handler) Requeue(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if _, err := h.pool.Exec(r.Context(),
		`UPDATE songs
		 SET status='queued',
		     position=(SELECT COALESCE(MAX(position),0)+1 FROM songs)
		 WHERE id=$1`, id); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao recolocar na fila")
		return
	}
	h.writeOne(w, r, id)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if _, err := h.pool.Exec(r.Context(), `DELETE FROM songs WHERE id=$1`, id); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao remover música")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (h *Handler) setBool(w http.ResponseWriter, r *http.Request, col string, val bool) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if _, err := h.pool.Exec(r.Context(),
		`UPDATE songs SET `+col+`=$2 WHERE id=$1`, id, val); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao atualizar")
		return
	}
	h.writeOne(w, r, id)
}

func (h *Handler) writeOne(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	row := h.pool.QueryRow(r.Context(), selectSongs+` WHERE s.id=$1`, id)
	s, err := scanSongRow(row)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar música")
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}

// rowScanner is satisfied by both pgx.Rows and pgx.Row.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanSong(rows rowScanner) (song, error) { return scanSongRow(rows) }

func scanSongRow(row rowScanner) (song, error) {
	var s song
	err := row.Scan(&s.ID, &s.YouTubeID, &s.URL, &s.Title, &s.ThumbnailURL, &s.Author,
		&s.PrendaID, &s.PrendaTitle, &s.PrendaDone, &s.Status, &s.RequestedBy, &s.CreatedAt)
	return s, err
}

func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "id inválido")
		return uuid.Nil, false
	}
	return id, true
}

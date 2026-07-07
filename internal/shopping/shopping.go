// Package shopping implements the shared shopping list: each item is a purchase
// paid by one user (with a mandatory receipt image). The total cost is split
// equally among all users and per-user balances (paid - share) are computed.
package shopping

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/guilherme/help-party/internal/auth"
	"github.com/guilherme/help-party/internal/httpx"
)

type Handler struct {
	pool       *pgxpool.Pool
	uploadsDir string
}

func New(pool *pgxpool.Pool, uploadsDir string) *Handler {
	_ = os.MkdirAll(uploadsDir, 0o755)
	return &Handler{pool: pool, uploadsDir: uploadsDir}
}

// Routes mounts the shopping endpoints. Any authenticated user can add/list;
// deletes are limited (owner or admin) inside the handler.
func (h *Handler) Routes(r chi.Router, _ func(http.Handler) http.Handler) {
	r.Get("/shopping", h.List)
	r.Post("/shopping", h.Create)
	r.Delete("/shopping/{id}", h.Delete)
}

// PublicRoutes mounts routes that must be reachable without auth (e.g. <img>
// tags loading receipt images). Filenames are unguessable UUIDs.
func (h *Handler) PublicRoutes(r chi.Router) {
	r.Get("/uploads/{name}", h.ServeReceipt)
}

type itemView struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	UnitCents  int64     `json:"unit_cents"`
	Quantity   int       `json:"quantity"`
	Unit       string    `json:"unit"`
	TotalCents int64     `json:"total_cents"`
	ItemURL    string    `json:"item_url"`
	ReceiptURL string    `json:"receipt_url"`
	PaidByID   uuid.UUID `json:"paid_by_id"`
	PaidByName string    `json:"paid_by_name"`
	CreatedAt  time.Time `json:"created_at"`
}

type balanceView struct {
	UserID       uuid.UUID `json:"user_id"`
	Name         string    `json:"name"`
	PaidCents    int64     `json:"paid_cents"`
	BalanceCents int64     `json:"balance_cents"`
}

type summary struct {
	Items      []itemView    `json:"items"`
	TotalCents int64         `json:"total_cents"`
	UserCount  int           `json:"user_count"`
	ShareCents int64         `json:"share_cents"`
	Balances   []balanceView `json:"balances"`
}

// List returns all items plus the split summary and per-user balances.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.pool.Query(ctx, `
		SELECT s.id, s.name, s.unit_cents, s.quantity, s.unit, s.item_path, s.receipt_path,
		       COALESCE(s.paid_by, '00000000-0000-0000-0000-000000000000'::uuid),
		       COALESCE(u.name, '—'), s.created_at
		FROM shopping_items s
		LEFT JOIN users u ON u.id = s.paid_by
		ORDER BY s.created_at DESC`)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar compras")
		return
	}
	defer rows.Close()

	items := []itemView{}
	paidByUser := map[uuid.UUID]int64{}
	var total int64
	for rows.Next() {
		var (
			it       itemView
			itemPath string
			path     string
		)
		if err := rows.Scan(&it.ID, &it.Name, &it.UnitCents, &it.Quantity, &it.Unit, &itemPath, &path,
			&it.PaidByID, &it.PaidByName, &it.CreatedAt); err != nil {
			httpx.Error(w, http.StatusInternalServerError, "erro ao ler item")
			return
		}
		it.TotalCents = it.UnitCents * int64(it.Quantity)
		if itemPath != "" {
			it.ItemURL = "/api/uploads/" + itemPath
		}
		if path != "" {
			it.ReceiptURL = "/api/uploads/" + path
		}
		total += it.TotalCents
		paidByUser[it.PaidByID] += it.TotalCents
		items = append(items, it)
	}
	rows.Close()

	// Split equally among ALL users.
	urows, err := h.pool.Query(ctx, `SELECT id, name FROM users ORDER BY name`)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao ler usuários")
		return
	}
	defer urows.Close()

	type u struct {
		id   uuid.UUID
		name string
	}
	var users []u
	for urows.Next() {
		var x u
		if urows.Scan(&x.id, &x.name) == nil {
			users = append(users, x)
		}
	}
	urows.Close()

	n := len(users)
	var share int64
	if n > 0 {
		share = total / int64(n)
	}
	balances := make([]balanceView, 0, n)
	for _, x := range users {
		paid := paidByUser[x.id]
		balances = append(balances, balanceView{
			UserID:       x.id,
			Name:         x.name,
			PaidCents:    paid,
			BalanceCents: paid - share,
		})
	}

	httpx.JSON(w, http.StatusOK, summary{
		Items:      items,
		TotalCents: total,
		UserCount:  n,
		ShareCents: share,
		Balances:   balances,
	})
}

// Create adds an item from a multipart form (name, value, quantity, receipt).
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	usr, _ := auth.FromContext(r.Context())

	if err := r.ParseMultipartForm(12 << 20); err != nil { // 12MB
		httpx.Error(w, http.StatusBadRequest, "formulário inválido")
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		httpx.Error(w, http.StatusBadRequest, "informe o nome do item")
		return
	}
	unitCents, err := parseCents(r.FormValue("value"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "valor inválido")
		return
	}
	qty, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("quantity")))
	if qty < 1 {
		qty = 1
	}
	unit := strings.TrimSpace(r.FormValue("unit"))
	if unit == "" {
		unit = "un"
	}

	// Both images are mandatory: photo of the item and the receipt.
	itemName, err := h.saveUpload(r, "item_image")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "a foto do item é obrigatória")
		return
	}
	receiptName, err := h.saveUpload(r, "receipt")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "o comprovante é obrigatório")
		return
	}

	var id uuid.UUID
	err = h.pool.QueryRow(r.Context(),
		`INSERT INTO shopping_items (name, unit_cents, quantity, unit, item_path, receipt_path, paid_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		name, unitCents, qty, unit, itemName, receiptName, usr.ID,
	).Scan(&id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao salvar item")
		return
	}
	h.List(w, r)
}

// Delete removes an item (only its payer or an admin).
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	usr, _ := auth.FromContext(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "id inválido")
		return
	}

	var (
		paidBy   *uuid.UUID
		path     string
		itemPath string
	)
	err = h.pool.QueryRow(r.Context(),
		`SELECT paid_by, receipt_path, item_path FROM shopping_items WHERE id=$1`, id).
		Scan(&paidBy, &path, &itemPath)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "item não encontrado")
		return
	}
	if !usr.IsAdmin && (paidBy == nil || *paidBy != usr.ID) {
		httpx.Error(w, http.StatusForbidden, "só quem adicionou (ou um admin) pode remover")
		return
	}

	if _, err := h.pool.Exec(r.Context(), `DELETE FROM shopping_items WHERE id=$1`, id); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao remover")
		return
	}
	for _, p := range []string{path, itemPath} {
		if p != "" {
			_ = os.Remove(filepath.Join(h.uploadsDir, p))
		}
	}
	h.List(w, r)
}

// ServeReceipt streams a stored receipt image.
func (h *Handler) ServeReceipt(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	// Guard against path traversal.
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join(h.uploadsDir, name))
}

// saveUpload reads a multipart file field and stores it, returning the filename.
func (h *Handler) saveUpload(r *http.Request, field string) (string, error) {
	file, header, err := r.FormFile(field)
	if err != nil {
		return "", err
	}
	defer file.Close()
	return h.saveReceipt(file, header.Filename)
}

func (h *Handler) saveReceipt(src io.Reader, origName string) (string, error) {
	ext := strings.ToLower(filepath.Ext(origName))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif", ".heic", ".pdf":
	default:
		ext = ".jpg"
	}
	filename := uuid.NewString() + ext
	dst, err := os.Create(filepath.Join(h.uploadsDir, filename))
	if err != nil {
		return "", err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, io.LimitReader(src, 12<<20)); err != nil {
		return "", err
	}
	return filename, nil
}

// parseCents turns "150", "150.50" or "150,50" into integer cents.
func parseCents(s string) (int64, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", "."))
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 {
		return 0, fmt.Errorf("invalid")
	}
	return int64(math.Round(f * 100)), nil
}

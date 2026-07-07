package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/guilherme/help-party/internal/auth"
	"github.com/guilherme/help-party/internal/config"
	"github.com/guilherme/help-party/internal/db"
	"github.com/guilherme/help-party/internal/game"
	"github.com/guilherme/help-party/internal/mailer"
	"github.com/guilherme/help-party/internal/music"
	"github.com/guilherme/help-party/internal/prendas"
	"github.com/guilherme/help-party/internal/queue"
	"github.com/guilherme/help-party/internal/rentals"
	"github.com/guilherme/help-party/internal/shopping"
	"github.com/guilherme/help-party/internal/users"
	"github.com/guilherme/help-party/web"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Println("migrations applied")

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	if err := queue.Migrate(ctx, pool); err != nil {
		log.Fatalf("river migrate: %v", err)
	}
	riverClient, err := queue.NewInsertClient(pool)
	if err != nil {
		log.Fatalf("river client: %v", err)
	}

	authSvc := auth.NewService(cfg.JWTSecret, cfg.JWTTTL)
	mail := mailer.New(cfg)
	usersH := users.New(pool, authSvc, mail, cfg)
	rentalsH := rentals.New(pool, riverClient)
	prendasH := prendas.New(pool)
	musicH := music.New(pool, cfg)
	gameH := game.New(pool)
	shoppingH := shopping.New(pool, cfg.UploadsDir)

	// Seed the configured admin users (idempotent).
	if err := usersH.Seed(ctx); err != nil {
		log.Printf("admin seed: %v", err)
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		// Only our own frontend (any *.justgui.dev) — plus localhost in dev — may
		// use the API from a browser.
		AllowOriginFunc:  allowedOrigin,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: false,
	}))

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		})

		// Public auth routes.
		r.Post("/auth/register", usersH.Register)
		r.Post("/auth/login", usersH.Login)
		r.Post("/auth/reset-request", usersH.ResetRequest)
		r.Post("/auth/reset", usersH.Reset)

		// Receipt images are served unauthenticated (UUID filenames) so <img>
		// tags can load them directly.
		shoppingH.PublicRoutes(r)

		// Protected routes. Handlers gate admin-only endpoints internally using
		// the RequireAdmin middleware passed to Routes().
		r.Group(func(r chi.Router) {
			r.Use(authSvc.RequireAuth)
			r.Get("/auth/me", usersH.Me)

			admin := authSvc.RequireAdmin
			usersH.AdminRoutes(r, admin)
			rentalsH.Routes(r, admin)
			prendasH.Routes(r, admin)
			musicH.Routes(r, admin)
			gameH.Routes(r, admin)
			shoppingH.Routes(r, admin)
		})
	})

	// Serve the embedded SPA for everything that is not /api.
	r.NotFound(web.Handler().ServeHTTP)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("api listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	log.Println("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

// allowedOrigin permits browser requests only from *.justgui.dev (the app's
// own frontend) plus localhost during development.
func allowedOrigin(_ *http.Request, origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if host == "justgui.dev" || strings.HasSuffix(host, ".justgui.dev") {
		return true
	}
	return host == "localhost" || host == "127.0.0.1"
}

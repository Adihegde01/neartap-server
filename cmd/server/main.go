package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	firebase "firebase.google.com/go/v4"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"google.golang.org/api/option"

	"github.com/neartap/server/config"
	"github.com/neartap/server/internal/handlers"
	appMiddleware "github.com/neartap/server/internal/middleware"
	"github.com/neartap/server/internal/store"
)

func main() {
	// ── Config ──────────────────────────────────────────────────────────────────
	cfg := config.Load()
	log.Printf("🚰 NearTap API starting [%s] on :%s", cfg.Env, cfg.Port)

	// ── Firebase App ────────────────────────────────────────────────────────────
	ctx := context.Background()
	var app *firebase.App
	var opts []option.ClientOption

	if fi, err := os.Stat(cfg.FirebaseCredentials); err == nil && !fi.IsDir() {
		opts = append(opts, option.WithCredentialsFile(cfg.FirebaseCredentials))
		log.Printf("[firebase] Using service account: %s", cfg.FirebaseCredentials)

		conf := &firebase.Config{ProjectID: cfg.FirebaseProjectID}
		var err error
		app, err = firebase.NewApp(ctx, conf, opts...)
		if err != nil {
			log.Fatalf("Failed to initialize Firebase App: %v", err)
		}
	} else {
		log.Printf("[firebase] No service account found at %s — running in DEMO mode", cfg.FirebaseCredentials)
	}

	// ── Store (Firestore / in-memory mock) ──────────────────────────────────────
	dataStore, err := store.New(ctx, app, cfg.FirebaseProjectID)
	if err != nil {
		log.Fatalf("Failed to initialise store: %v", err)
	}
	defer dataStore.Close()

	// ── Auth middleware ─────────────────────────────────────────────────────────
	authMW := appMiddleware.NewFirebaseAuth(app)

	// ── Handlers ────────────────────────────────────────────────────────────────
	tapHandler := handlers.NewTapHandler(dataStore)

	// ── Router ──────────────────────────────────────────────────────────────────
	r := chi.NewRouter()

	// Global middleware
	r.Use(chiMiddleware.RealIP)
	r.Use(appMiddleware.Logger)
	r.Use(appMiddleware.Recoverer)
	r.Use(chiMiddleware.Timeout(30 * time.Second))

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// ── Routes ──────────────────────────────────────────────────────────────────
	r.Get("/health", handlers.HealthHandler)
	r.NotFound(handlers.NotFoundHandler)

	r.Mount("/api/taps", tapHandler.Routes(authMW))

	// ── API info route ───────────────────────────────────────────────────────────
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
  "service": "NearTap API",
  "version": "1.0.0",
  "docs": "https://github.com/neartap/server",
  "endpoints": {
    "GET  /health":               "Health check",
    "GET  /api/taps":             "List all taps (optional ?lat=&lng=&radius=)",
    "GET  /api/taps/nearby":      "Nearby taps ?lat=&lng=&radius=",
    "GET  /api/taps/:id":         "Get tap by ID",
    "POST /api/taps":             "Create tap (auth required)",
    "POST /api/taps/:id/confirm": "Confirm tap works (auth required)",
    "POST /api/taps/:id/report":  "Report issue (auth required)"
  }
}`)
	})

	// ── HTTP Server ─────────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start in goroutine so we can handle shutdown signals
	go func() {
		log.Printf("✅ Server listening on http://localhost:%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// ── Graceful shutdown ───────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down server…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
	log.Println("👋 Server exited cleanly")
}

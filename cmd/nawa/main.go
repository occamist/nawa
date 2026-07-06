package main

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/client"
	"github.com/google/uuid"

	"github.com/occamist/nawa/auth"
	"github.com/occamist/nawa/config"
	"github.com/occamist/nawa/handlers"
	"github.com/occamist/nawa/ratelimiter"
	"github.com/occamist/nawa/store"
	"github.com/occamist/nawa/webdist"
)

const (
	requestTimeout  = 30 * time.Second
	shutdownTimeout = 5 * time.Second
)

var loginRateLimit = ratelimiter.Config{
	MaxFailures: 5,
	Window:      15 * time.Minute,
	BlockFor:    15 * time.Minute,
	MaxKeys:     10000,
	MaxKeyLen:   256,
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()
	db, err := store.Connect(ctx, "nawa.db")
	if err != nil {
		slog.Error("db connect failed", "err", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	exists, err := store.UserExists(ctx, db, cfg.AdminUsername)
	if err != nil {
		slog.Error("check admin user failed", "err", err)
		os.Exit(1)
	}
	if !exists {
		adminPassword := cfg.AdminPassword
		if adminPassword == "" {
			adminPassword = uuid.NewString()
			slog.Info("generated admin credentials — SAVE THESE, SHOWN ONLY ONCE", "username", cfg.AdminUsername, "password", adminPassword)
		}
		if err := store.Seed(ctx, db, cfg.AdminUsername, adminPassword); err != nil {
			slog.Error("seed failed", "err", err)
			os.Exit(1)
		}
	}

	dc, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		slog.Error("docker client failed", "err", err)
		os.Exit(1)
	}
	defer func() { _ = dc.Close() }()

	mux := http.NewServeMux()
	static, err := fs.Sub(webdist.FS, "dist")
	if err != nil {
		slog.Error("static files has no dist directory", "err", err)
		os.Exit(1)
	}
	mux.Handle("/", http.FileServer(http.FS(static)))

	mux.HandleFunc("GET /healthz", handlers.Healthz())

	ratelimiter := ratelimiter.New(loginRateLimit)
	mux.HandleFunc("POST /api/v1/auth/login", handlers.Login(db, cfg, ratelimiter))
	mux.HandleFunc("POST /api/v1/auth/logout", handlers.Logout(cfg))

	protected := http.NewServeMux()
	protected.HandleFunc("GET /api/v1/containers", handlers.ListContainers(dc))
	protected.HandleFunc("POST /api/v1/containers/{id}/start", handlers.StartContainer(dc))
	protected.HandleFunc("POST /api/v1/containers/{id}/stop", handlers.StopContainer(dc))
	protected.HandleFunc("DELETE /api/v1/containers/{id}", handlers.RemoveContainer(dc))
	protected.HandleFunc("GET /api/v1/containers/{id}/logs", handlers.StreamContainerLogs(dc))
	protected.HandleFunc("GET /api/v1/images", handlers.ListImages(dc))
	protected.HandleFunc("POST /api/v1/images/pull", handlers.PullImage(dc))
	protected.HandleFunc("DELETE /api/v1/images/{id}", handlers.RemoveImage(dc))
	protected.HandleFunc("POST /api/v1/images/prune", handlers.PruneImages(dc))

	mux.Handle("/api/v1/", auth.Middleware(cfg, protected))

	addr := net.JoinHostPort(cfg.Host, cfg.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           requestTimeoutMiddleware(requestTimeout, mux),
		ReadHeaderTimeout: requestTimeout,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go func() {
		slog.Info("nawa listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	slog.Info("shutting down")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); !errors.Is(err, http.ErrServerClosed) && err != nil {
		slog.Error("shutdown error", "err", err)
		os.Exit(1)
	}
	slog.Info("shutdown complete")
}

func requestTimeoutMiddleware(timeout time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isStreaming := r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/containers/") && strings.HasSuffix(r.URL.Path, "/logs")
		if isStreaming {
			next.ServeHTTP(w, r)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

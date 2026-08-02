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
	"syscall"
	"time"

	"github.com/docker/docker/client"
	"github.com/google/uuid"

	"github.com/occamist/nawa/config"
	"github.com/occamist/nawa/handlers"
	"github.com/occamist/nawa/hoststats"
	"github.com/occamist/nawa/ratelimiter"
	"github.com/occamist/nawa/router"
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

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

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

	sampler := hoststats.NewSampler(cfg.DiskPath, time.Second)
	go sampler.Run(ctx)

	mux := http.NewServeMux()
	static, err := fs.Sub(webdist.FS, "dist")
	if err != nil {
		slog.Error("static files has no dist directory", "err", err)
		os.Exit(1)
	}

	ratelimiter := ratelimiter.New(loginRateLimit)

	authMW := router.AuthMiddleware(cfg)
	timeoutMW := router.TimeoutMiddleware(requestTimeout)

	root := router.New(mux)
	root.Handle("/", http.FileServer(http.FS(static)))
	root.HandleFunc("GET /healthz", handlers.Healthz())

	root.Route("/api/v1", func(api *router.Group) {
		api.HandleFunc("POST /auth/login", handlers.Login(db, cfg, ratelimiter))
		api.HandleFunc("POST /auth/logout", handlers.Logout(cfg))

		api.Route("", func(protected *router.Group) {
			protected.Use(authMW, timeoutMW)
			protected.HandleFunc("GET /containers", handlers.ListContainers(dc))
			protected.HandleFunc("POST /containers/{id}/start", handlers.StartContainer(dc))
			protected.HandleFunc("POST /containers/{id}/stop", handlers.StopContainer(dc))
			protected.HandleFunc("DELETE /containers/{id}", handlers.RemoveContainer(dc))
			protected.HandleFunc("GET /images", handlers.ListImages(dc))
			protected.HandleFunc("POST /images/pull", handlers.PullImage(dc))
			protected.HandleFunc("DELETE /images/{id}", handlers.RemoveImage(dc))
			protected.HandleFunc("POST /images/prune", handlers.PruneImages(dc))
		})

		api.Route("", func(streaming *router.Group) {
			streaming.Use(authMW) // no timeout, connections are meant to stay open
			streaming.HandleFunc("GET /containers/{id}/logs", handlers.StreamContainerLogs(dc))
			streaming.HandleFunc("GET /host/stats", handlers.StreamHostStats(sampler))
		})
	})

	addr := net.JoinHostPort(cfg.Host, cfg.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: requestTimeout,
	}

	go func() {
		slog.Info("nawa listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	slog.Info("shutting down")
	// detach from ctx's cancellation so shutdown gets its own timeout, not zero time`
	shutdownCtx, shutdownCancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); !errors.Is(err, http.ErrServerClosed) && err != nil {
		slog.Error("shutdown error", "err", err)
		os.Exit(1)
	}
	slog.Info("shutdown complete")
}

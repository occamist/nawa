package main

import (
	"context"
	"errors"
	"fmt"
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
	if err := run(); err != nil {
		slog.Error("run failed", "err", err)
		os.Exit(1)
	}
}

func run() error {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config load failed: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	s, err := store.New(ctx, "nawa.db")
	if err != nil {
		return fmt.Errorf("db connect failed: %v", err)
	}
	defer func() { _ = s.Close() }()

	exists, err := s.UserExists(ctx, cfg.AdminUsername)
	if err != nil {
		return fmt.Errorf("check admin user failed: %v", err)
	}
	if !exists {
		adminPassword := cfg.AdminPassword
		if adminPassword == "" {
			adminPassword = uuid.NewString()
			slog.Info("generated admin credentials — SAVE THESE, SHOWN ONLY ONCE", "username", cfg.AdminUsername, "password", adminPassword)
		}
		if err := s.Seed(ctx, cfg.AdminUsername, adminPassword); err != nil {
			return fmt.Errorf("seed failed: %v", err)
		}
	}

	dc, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("docker client failed: %v", err)
	}
	defer func() { _ = dc.Close() }()

	sampler := hoststats.NewSampler(cfg.DiskPath, time.Second)
	go sampler.Run(ctx)

	mux := http.NewServeMux()
	static, err := fs.Sub(webdist.FS, "dist")
	if err != nil {
		return fmt.Errorf("static files has no dist directory: %v", err)
	}

	ratelimiter := ratelimiter.New(loginRateLimit)

	authMW := router.AuthMiddleware(cfg)
	timeoutMW := router.TimeoutMiddleware(requestTimeout)

	root := router.New(mux)
	root.Handle("/", http.FileServer(http.FS(static)))
	root.HandleFunc("GET /healthz", handlers.Healthz())

	root.Route("/api/v1", func(api *router.Group) {
		api.HandleFunc("POST /auth/login", handlers.Login(s, cfg, ratelimiter))
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
			streaming.Use(authMW) // no timeout middleware, connections are meant to stay open
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
			slog.Error("server failed to listen", "err", err)
		}
	}()

	<-ctx.Done()

	slog.Info("gracefully shutting down")
	// detach from ctx's cancellation so shutdown gets its own timeout, not zero time
	shutdownCtx, shutdownCancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); !errors.Is(err, http.ErrServerClosed) && err != nil {
		return fmt.Errorf("server failed to shutdown: %v", err)
	}
	slog.Info("graceful shutdown complete")
	return nil
}

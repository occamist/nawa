package handlers

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"golang.org/x/crypto/bcrypt"

	"github.com/occamist/nawa/config"
	"github.com/occamist/nawa/ratelimiter"
)

const (
	tokenTTL = 24 * time.Hour

	// Keep unknown-user login attempts on the bcrypt path to reduce username
	// enumeration through the large timing gap between "no row" and bcrypt.
	dummyPasswordHash = "$2a$10$7EqJtq98hPqEX7fNZaFWoO1bVJv2BHLHgyV4YcN4rwrL7GxKi0mEa" //nolint:gosec // this is not real credential
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func Login(db *sql.DB, cfg config.Config, limiter *ratelimiter.Limiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const oneMB = 1 << 20
		var req loginRequest

		r.Body = http.MaxBytesReader(w, r.Body, oneMB)
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
			http.Error(w, "username and password required", http.StatusBadRequest)
			return
		}

		ipKey := "ip:" + clientIP(r)
		usernameKey := "user:" + normalizeUsername(req.Username)
		if !limiter.Allow(ipKey) || !limiter.Allow(usernameKey) {
			http.Error(w, "too many login attempts", http.StatusTooManyRequests)
			return
		}

		var hash string
		err := db.QueryRowContext(r.Context(),
			"SELECT password_hash FROM users WHERE username = ?", req.Username,
		).Scan(&hash)
		userExists := true
		if err == sql.ErrNoRows {
			userExists = false
			hash = dummyPasswordHash
		} else if err != nil {
			slog.Error("query user", "err", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil || !userExists {
			limiter.RecordFailure(ipKey)
			limiter.RecordFailure(usernameKey)
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}

		limiter.Reset(ipKey)
		limiter.Reset(usernameKey)

		token, err := jwt.NewBuilder().
			Subject(req.Username).
			IssuedAt(time.Now()).
			Expiration(time.Now().Add(tokenTTL)).
			Build()
		if err != nil {
			slog.Error("build token", "err", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		signed, err := jwt.Sign(token, jwt.WithKey(jwa.HS256(), []byte(cfg.JWTSecret)))
		if err != nil {
			slog.Error("sign token", "err", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{ //nolint:gosec // secure is configurable
			Name:     cfg.CookieName,
			Value:    string(signed),
			Path:     "/",
			HttpOnly: true,
			Secure:   cfg.CookieSecure,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   int(tokenTTL.Seconds()),
		})

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			slog.Error("logout", "err", err)
			http.Error(w, "Server encountered an error", http.StatusInternalServerError)
		}
	}
}

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return host
	}
	return ip.String()
}

func Logout(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{ //nolint:gosec // secure is configurable
			Name:     cfg.CookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   cfg.CookieSecure,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   -1,
		})

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			slog.Error("logout", "err", err)
			http.Error(w, "Server encountered an error", http.StatusInternalServerError)
		}
	}
}

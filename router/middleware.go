package router

import (
	"context"
	"net/http"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwt"

	"github.com/occamist/nawa/config"
)

type Middleware func(http.Handler) http.Handler

// TimeoutMiddleware times out the requests that take longer than certain duration
func TimeoutMiddleware(timeout time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AuthMiddleware validates the JWT cookie and rejects unauthenticated requests.
func AuthMiddleware(cfg config.Config) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(cfg.CookieName)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			_, err = jwt.Parse([]byte(cookie.Value),
				jwt.WithKey(jwa.HS256(), []byte(cfg.JWTSecret)),
				jwt.WithValidate(true),
			)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

package auth

import (
	"net/http"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwt"

	"github.com/occamist/nawa/config"
)

const CookieName = "nawa_token"

// Middleware validates the JWT cookie and rejects unauthenticated requests.
func Middleware(cfg config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(CookieName)
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

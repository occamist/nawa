package router

import (
	"net/http"
	"slices"
	"strings"
)

type Group struct {
	mux        *http.ServeMux
	prefix     string
	middleware []Middleware
}

func New(mux *http.ServeMux) *Group {
	return &Group{mux: mux}
}

// Use appends middleware applied to every route registered on this group
// (and any sub-group created via Route afterward). Runs in the order added.
func (g *Group) Use(mw ...Middleware) {
	g.middleware = append(g.middleware, mw...)
}

// Route creates a sub-group nested under prefix
func (g *Group) Route(prefix string, fn func(*Group)) {
	fn(&Group{
		mux:        g.mux,
		prefix:     g.prefix + prefix,
		middleware: append([]Middleware{}, g.middleware...),
	})
}

func (g *Group) HandleFunc(pattern string, h http.HandlerFunc) {
	g.Handle(pattern, h)
}

func (g *Group) Handle(pattern string, h http.Handler) {
	for _, m := range slices.Backward(g.middleware) {
		h = m(h)
	}
	g.mux.Handle(withPrefix(g.prefix, pattern), h)
}

// withPrefix inserts prefix between the method and path
// e.g. ("/api/v1", "GET /containers") -> "GET /api/v1/containers".
func withPrefix(prefix, pattern string) string {
	if method, path, ok := strings.Cut(pattern, " "); ok {
		return method + " " + prefix + path
	}
	return prefix + pattern
}

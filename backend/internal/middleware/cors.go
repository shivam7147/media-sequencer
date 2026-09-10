// Package middleware holds cross-cutting HTTP middleware.
package middleware

import (
	"net/http"
	"strings"
)

// CORS allows only origins in allowedOrigins — a comma-separated list, so
// the same deployed backend can serve both the deployed frontend and
// http://localhost:5173 during development. A request's Origin is matched
// exactly against that list and echoed back verbatim only on a match;
// never "*", and never an origin that isn't an exact match (an unmatched
// or absent Origin gets no Access-Control-Allow-Origin header at all,
// which is what makes the browser block it). It must run ahead of every
// route, including /api/events: SSE requests are still subject to CORS
// like any other cross-origin fetch.
func CORS(allowedOrigins string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool)
	for _, origin := range strings.Split(allowedOrigins, ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			allowed[origin] = true
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// The response now depends on the request's Origin header, so
			// caches (browser, CDN, Render's own proxy) need to know that —
			// without Vary, a cached response for one origin could be served
			// to a different one.
			w.Header().Add("Vary", "Origin")

			if origin := r.Header.Get("Origin"); allowed[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

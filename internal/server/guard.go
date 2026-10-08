package server

import (
	"crypto/subtle"
	"fmt"
	"mime"
	"net/http"
	"strings"
)

const cookieName = "dungeon_session"

// guard is the trust boundary for a localhost server that can drive
// terminals. It defends against pages in the user's browser (cross-origin
// requests, DNS rebinding) and against other local processes without the
// token.
//
//   - Host must be one of our own loopback names, which blocks DNS rebinding.
//   - A request carrying an Origin must carry ours.
//   - Writes must be application/json, so a cross-site form cannot reach them.
//   - /api/ needs the session cookie or a bearer token.
type guard struct {
	token string
	hosts map[string]bool
}

func newGuard(port int, token string) *guard {
	h := map[string]bool{}
	for _, name := range []string{"127.0.0.1", "localhost", "[::1]"} {
		h[fmt.Sprintf("%s:%d", name, port)] = true
	}
	return &guard{token: token, hosts: h}
}

func (g *guard) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w)

		if !g.hosts[r.Host] {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		if o := r.Header.Get("Origin"); o != "" && !g.originOK(o) {
			http.Error(w, "forbidden origin", http.StatusForbidden)
			return
		}
		// Browsers always send Origin on websocket upgrades. Requiring it
		// closes cross-site websocket hijacking, which CORS does not cover.
		if isWebsocket(r) && r.Header.Get("Origin") == "" {
			http.Error(w, "origin required", http.StatusForbidden)
			return
		}
		if unsafeMethod(r.Method) && !isJSON(r.Header.Get("Content-Type")) {
			http.Error(w, "content type must be application/json", http.StatusUnsupportedMediaType)
			return
		}

		// A valid ?token= on a page load becomes a cookie, then the URL is
		// cleaned so the token does not linger in history or Referer.
		if r.Method == http.MethodGet && g.tokenOK(r.URL.Query().Get("token")) {
			http.SetCookie(w, &http.Cookie{
				Name:     cookieName,
				Value:    g.token,
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteStrictMode,
			})
			http.Redirect(w, r, r.URL.Path, http.StatusSeeOther)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/api/") && !g.authed(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (g *guard) originOK(origin string) bool {
	host, ok := strings.CutPrefix(origin, "http://")
	return ok && g.hosts[host]
}

func (g *guard) tokenOK(got string) bool {
	return got != "" && subtle.ConstantTimeCompare([]byte(got), []byte(g.token)) == 1
}

func (g *guard) authed(r *http.Request) bool {
	if c, err := r.Cookie(cookieName); err == nil && g.tokenOK(c.Value) {
		return true
	}
	if b, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
		return g.tokenOK(b)
	}
	return false
}

func isWebsocket(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

func unsafeMethod(m string) bool {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	return true
}

func isJSON(ct string) bool {
	mt, _, err := mime.ParseMediaType(ct)
	return err == nil && mt == "application/json"
}

func setSecurityHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'; base-uri 'none'")
}

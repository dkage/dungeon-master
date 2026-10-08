package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testPort  = 7878
	testToken = "secret-token"
)

func newHandler() http.Handler {
	return Handler(Config{Token: testToken, Version: "test"}, testPort)
}

func do(t *testing.T, method, target string, mutate func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(""))
	req.Host = "127.0.0.1:7878"
	if mutate != nil {
		mutate(req)
	}
	rec := httptest.NewRecorder()
	newHandler().ServeHTTP(rec, req)
	return rec
}

func TestHostGuard(t *testing.T) {
	for host, want := range map[string]int{
		"127.0.0.1:7878": http.StatusOK,
		"localhost:7878": http.StatusOK,
		"[::1]:7878":     http.StatusOK,
		"evil.test:7878": http.StatusForbidden, // DNS rebinding
		"127.0.0.1:9999": http.StatusForbidden,
		"127.0.0.1":      http.StatusForbidden,
	} {
		rec := do(t, "GET", "/", func(r *http.Request) { r.Host = host })
		if rec.Code != want {
			t.Errorf("host %q: got %d, want %d", host, rec.Code, want)
		}
	}
}

func TestOriginGuard(t *testing.T) {
	for origin, want := range map[string]int{
		"":                       http.StatusOK,
		"http://127.0.0.1:7878":  http.StatusOK,
		"http://localhost:7878":  http.StatusOK,
		"http://evil.test":       http.StatusForbidden,
		"https://127.0.0.1:7878": http.StatusForbidden,
		"null":                   http.StatusForbidden,
	} {
		rec := do(t, "GET", "/", func(r *http.Request) {
			if origin != "" {
				r.Header.Set("Origin", origin)
			}
		})
		if rec.Code != want {
			t.Errorf("origin %q: got %d, want %d", origin, rec.Code, want)
		}
	}
}

func TestAPIRequiresAuth(t *testing.T) {
	if rec := do(t, "GET", "/api/health", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no credentials: got %d", rec.Code)
	}
	rec := do(t, "GET", "/api/health", func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer wrong")
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong bearer: got %d", rec.Code)
	}
	rec = do(t, "GET", "/api/health", func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+testToken)
	})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"version":"test"`) {
		t.Fatalf("bearer: got %d %s", rec.Code, rec.Body)
	}
	rec = do(t, "GET", "/api/health", func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: cookieName, Value: testToken})
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("cookie: got %d", rec.Code)
	}
}

func TestTokenExchange(t *testing.T) {
	rec := do(t, "GET", "/?token="+testToken, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("got %d, Location %q", rec.Code, rec.Header().Get("Location"))
	}
	var c *http.Cookie
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == cookieName {
			c = ck
		}
	}
	if c == nil || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Value != testToken {
		t.Fatalf("bad cookie: %+v", c)
	}
	// A wrong token must not set anything.
	rec = do(t, "GET", "/?token=nope", nil)
	if len(rec.Result().Cookies()) != 0 {
		t.Fatal("wrong token set a cookie")
	}
}

func TestWritesRequireJSON(t *testing.T) {
	for ct, want := range map[string]int{
		"application/x-www-form-urlencoded": http.StatusUnsupportedMediaType,
		"text/plain":                        http.StatusUnsupportedMediaType,
		"":                                  http.StatusUnsupportedMediaType,
		"application/json":                  http.StatusNotFound, // passes the guard, no such route
		"application/json; charset=utf-8":   http.StatusNotFound,
	} {
		rec := do(t, "POST", "/api/anything", func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer "+testToken)
			if ct != "" {
				r.Header.Set("Content-Type", ct)
			}
		})
		if rec.Code != want {
			t.Errorf("content type %q: got %d, want %d", ct, rec.Code, want)
		}
	}
}

func TestFallbackPageAndHeaders(t *testing.T) {
	rec := do(t, "GET", "/", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Dungeon Master") {
		t.Fatalf("got %d %q", rec.Code, rec.Body)
	}
	if rec.Header().Get("Content-Security-Policy") == "" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing security headers")
	}
}

func TestNewRejectsNonLoopback(t *testing.T) {
	for addr, ok := range map[string]bool{
		"127.0.0.1:0": true,
		"localhost:0": true,
		"[::1]:0":     true,
		"0.0.0.0:0":   false,
		":0":          false,
		"10.0.0.5:0":  false,
	} {
		_, err := New(Config{Addr: addr, Token: "t"})
		if (err == nil) != ok {
			t.Errorf("addr %q: err=%v, want ok=%v", addr, err, ok)
		}
	}
	if _, err := New(Config{Addr: "127.0.0.1:0"}); err == nil {
		t.Error("empty token accepted")
	}
}

func TestWebsocketUpgradeRequiresOrigin(t *testing.T) {
	upgrade := func(r *http.Request) {
		r.Header.Set("Upgrade", "websocket")
		r.Header.Set("Authorization", "Bearer "+testToken)
	}
	if rec := do(t, "GET", "/api/sessions/s1/term", upgrade); rec.Code != http.StatusForbidden {
		t.Fatalf("upgrade without Origin: got %d", rec.Code)
	}
	rec := do(t, "GET", "/api/sessions/s1/term", func(r *http.Request) {
		upgrade(r)
		r.Header.Set("Origin", "http://127.0.0.1:7878")
	})
	if rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized {
		t.Fatalf("legitimate upgrade blocked by guard: %d", rec.Code)
	}
}

func TestContentSecurityPolicy(t *testing.T) {
	csp := do(t, "GET", "/", nil).Header().Get("Content-Security-Policy")
	directives := map[string]string{}
	for _, d := range strings.Split(csp, ";") {
		name, val, _ := strings.Cut(strings.TrimSpace(d), " ")
		directives[name] = val
	}
	// Scripts and everything else default to our own origin only.
	if directives["default-src"] != "'self'" {
		t.Errorf("default-src %q", directives["default-src"])
	}
	if v, ok := directives["script-src"]; ok && strings.Contains(v, "unsafe") {
		t.Errorf("script-src must not allow unsafe sources: %q", v)
	}
	if strings.Contains(directives["default-src"], "unsafe") {
		t.Error("default-src must not allow unsafe sources")
	}
	// xterm.js injects <style> elements; without this the terminal renders
	// with the wrong font and no selection highlight.
	if !strings.Contains(directives["style-src"], "'unsafe-inline'") {
		t.Errorf("style-src must allow inline styles for the terminal: %q", directives["style-src"])
	}
	// The terminal websocket must stay reachable, and only on loopback.
	// ('self' already covers an IPv6 loopback host; bracketed IPv6 sources are
	// invalid in CSP and make browsers log a warning, so none is listed.)
	if !strings.Contains(directives["connect-src"], "ws://127.0.0.1:*") ||
		strings.Contains(directives["connect-src"], "*.") || strings.Contains(directives["connect-src"], "[") {
		t.Errorf("connect-src %q", directives["connect-src"])
	}
	for _, must := range []string{"frame-ancestors", "base-uri", "object-src", "form-action"} {
		if directives[must] != "'none'" {
			t.Errorf("%s = %q, want 'none'", must, directives[must])
		}
	}
}

func TestTokenPersistsBetweenRuns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "token")
	first, created, err := LoadOrCreateToken(path, false)
	if err != nil || !created || !tokenPattern.MatchString(first) {
		t.Fatalf("first: %q created=%v err=%v", first, created, err)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Fatalf("token file mode %v", fi.Mode().Perm())
	}
	second, created, err := LoadOrCreateToken(path, false)
	if err != nil || created || second != first {
		t.Fatalf("second run should reuse the token: %q created=%v err=%v", second, created, err)
	}
}

func TestTokenRotation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	first, _, _ := LoadOrCreateToken(path, false)
	rotated, created, err := LoadOrCreateToken(path, true)
	if err != nil || !created || rotated == first {
		t.Fatalf("rotate: %q created=%v err=%v", rotated, created, err)
	}
	if again, _, _ := LoadOrCreateToken(path, false); again != rotated {
		t.Fatal("the rotated token was not stored")
	}
}

func TestInvalidStoredTokenIsReplaced(t *testing.T) {
	for name, content := range map[string]string{
		"empty": "", "short": "abc123", "not hex": strings.Repeat("z", 64),
		"too long": strings.Repeat("a", 65), "newlines only": "\n\n",
	} {
		path := filepath.Join(t.TempDir(), "token")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		tok, created, err := LoadOrCreateToken(path, false)
		if err != nil || !created || !tokenPattern.MatchString(tok) {
			t.Errorf("%s: %q created=%v err=%v", name, tok, created, err)
		}
	}
	// A valid token with a trailing newline (hand-edited) is accepted.
	path := filepath.Join(t.TempDir(), "token")
	good := strings.Repeat("ab", 32)
	_ = os.WriteFile(path, []byte(good+"\n"), 0o600)
	if tok, created, _ := LoadOrCreateToken(path, false); created || tok != good {
		t.Errorf("valid token rejected: %q %v", tok, created)
	}
}

func TestTokenFileUnreadableIsAnError(t *testing.T) {
	dir := t.TempDir()
	// A directory where the file should be cannot be read as a token.
	if err := os.Mkdir(filepath.Join(dir, "token"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadOrCreateToken(filepath.Join(dir, "token"), false); err == nil {
		t.Fatal("expected an error")
	}
}

func TestSessionCookieOutlivesTheBrowser(t *testing.T) {
	rec := do(t, "GET", "/?token="+testToken, nil)
	for _, c := range rec.Result().Cookies() {
		if c.Name == cookieName {
			if c.MaxAge != cookieMaxAge || c.MaxAge < 24*60*60 || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode {
				t.Fatalf("cookie: %+v", c)
			}
			return
		}
	}
	t.Fatal("no session cookie set")
}

func TestCheckAddr(t *testing.T) {
	for addr, ok := range map[string]bool{
		"127.0.0.1:7878": true, "localhost:0": true, "[::1]:9": true,
		"0.0.0.0:80": false, ":80": false, "192.168.1.5:80": false, "nonsense": false, "": false,
	} {
		if err := CheckAddr(addr); (err == nil) != ok {
			t.Errorf("%q: err=%v, want ok=%v", addr, err, ok)
		}
	}
}

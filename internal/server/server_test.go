package server

import (
	"net/http"
	"net/http/httptest"
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
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Dungeon") {
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

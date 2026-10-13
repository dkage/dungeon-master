package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var tinyPNG = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00")

func uploadBody(name string, data []byte) map[string]any {
	return map[string]any{"name": name, "data": base64.StdEncoding.EncodeToString(data)}
}

func TestUploadStoresTheFileAndSessionEndRemovesIt(t *testing.T) {
	e := startServer(t, nil)
	sj := e.create(map[string]any{"shell": true})
	resp, out := e.req("POST", "/api/sessions/"+sj.ID+"/uploads", uploadBody("shot.png", tinyPNG), nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload: %d %s", resp.StatusCode, out)
	}
	var got uploadResp
	_ = json.Unmarshal(out, &got)
	if got.Kind != "image" || !strings.HasSuffix(got.Path, "-shot.png") {
		t.Fatalf("response = %+v", got)
	}
	if want := filepath.Join(e.cfg.StateDir, "uploads", sj.ID); filepath.Dir(got.Path) != want {
		t.Errorf("stored in %q, want %q", filepath.Dir(got.Path), want)
	}
	if b, err := os.ReadFile(got.Path); err != nil || !bytes.Equal(b, tinyPNG) {
		t.Fatalf("stored file: %v", err)
	}

	// Removing the session removes its uploads.
	if resp, _ := e.req("DELETE", "/api/sessions/"+sj.ID, nil, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: %d", resp.StatusCode)
	}
	if _, err := os.Stat(got.Path); !os.IsNotExist(err) {
		t.Error("upload should be removed with its session")
	}
}

func TestUploadsGoWhenTheProcessExits(t *testing.T) {
	e := startServer(t, nil)
	sj := e.create(map[string]any{"shell": true})
	_, out := e.req("POST", "/api/sessions/"+sj.ID+"/uploads", uploadBody("a.png", tinyPNG), nil)
	var got uploadResp
	_ = json.Unmarshal(out, &got)
	live, _ := e.cfg.Sessions.Get(sj.ID)
	live.Kill()
	<-live.Done()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(got.Path); os.IsNotExist(err) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(got.Path); !os.IsNotExist(err) {
		t.Error("upload should be removed when the session ends")
	}
	// An ended session takes no more files.
	if resp, _ := e.req("POST", "/api/sessions/"+sj.ID+"/uploads", uploadBody("b.png", tinyPNG), nil); resp.StatusCode != http.StatusConflict && resp.StatusCode != http.StatusNotFound {
		t.Errorf("upload to an ended session: %d", resp.StatusCode)
	}
}

func TestUploadRefusesBadRequests(t *testing.T) {
	e := startServer(t, nil)
	sj := e.create(map[string]any{"shell": true})
	path := "/api/sessions/" + sj.ID + "/uploads"
	huge := append(append([]byte{}, tinyPNG...), make([]byte, 11<<20)...)

	for name, tc := range map[string]struct {
		path   string
		body   any
		mutate func(*http.Request)
		want   int
	}{
		"no token":         {path, uploadBody("a.png", tinyPNG), func(r *http.Request) { r.Header.Del("Authorization") }, http.StatusUnauthorized},
		"wrong token":      {path, uploadBody("a.png", tinyPNG), func(r *http.Request) { r.Header.Set("Authorization", "Bearer nope") }, http.StatusUnauthorized},
		"foreign origin":   {path, uploadBody("a.png", tinyPNG), func(r *http.Request) { r.Header.Set("Origin", "http://evil.test") }, http.StatusForbidden},
		"foreign host":     {path, uploadBody("a.png", tinyPNG), func(r *http.Request) { r.Host = "evil.test" }, http.StatusForbidden},
		"form content":     {path, uploadBody("a.png", tinyPNG), func(r *http.Request) { r.Header.Set("Content-Type", "multipart/form-data") }, http.StatusUnsupportedMediaType},
		"unknown session":  {"/api/sessions/nope/uploads", uploadBody("a.png", tinyPNG), nil, http.StatusNotFound},
		"traversal in id":  {"/api/sessions/..%2f..%2fx/uploads", uploadBody("a.png", tinyPNG), nil, http.StatusNotFound},
		"not base64":       {path, map[string]any{"name": "a.png", "data": "***"}, nil, http.StatusBadRequest},
		"unknown field":    {path, map[string]any{"name": "a.png", "data": "AAAA", "dir": "/etc"}, nil, http.StatusRequestEntityTooLarge},
		"empty":            {path, uploadBody("a.png", nil), nil, http.StatusBadRequest},
		"executable":       {path, uploadBody("a.png", []byte("\x7fELF\x02\x01\x01\x00")), nil, http.StatusUnsupportedMediaType},
		"larger than 10MB": {path, uploadBody("a.png", huge), nil, http.StatusRequestEntityTooLarge},
	} {
		resp, out := e.req("POST", tc.path, tc.body, tc.mutate)
		if resp.StatusCode != tc.want {
			t.Errorf("%s: %d %s, want %d", name, resp.StatusCode, out, tc.want)
		}
	}
	if entries, _ := os.ReadDir(filepath.Join(e.cfg.StateDir, "uploads", sj.ID)); len(entries) != 0 {
		t.Errorf("refused requests left %d files", len(entries))
	}
}

func TestUploadNameCannotEscape(t *testing.T) {
	e := startServer(t, nil)
	sj := e.create(map[string]any{"shell": true})
	_, out := e.req("POST", "/api/sessions/"+sj.ID+"/uploads", uploadBody("../../../escape.png", tinyPNG), nil)
	var got uploadResp
	_ = json.Unmarshal(out, &got)
	if want := filepath.Join(e.cfg.StateDir, "uploads", sj.ID); filepath.Dir(got.Path) != want {
		t.Errorf("stored in %q, want inside %q", filepath.Dir(got.Path), want)
	}
}

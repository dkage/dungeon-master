package gatekeeper

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// shortTmp is a short temporary directory: socket paths have a length limit,
// and t.TempDir() paths on macOS can be too long.
func shortTmp(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("/tmp", "gk")
	if err != nil {
		t.Skip("no /tmp")
	}
	t.Cleanup(func() { os.RemoveAll(d) })
	return d
}

func TestListenSocketServesHTTPAndCleansUp(t *testing.T) {
	t.Setenv("TMPDIR", shortTmp(t))
	t.Setenv("TEMP", os.Getenv("TMPDIR"))
	ln, path, err := ListenSocket()
	if err != nil {
		t.Fatal(err)
	}
	go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "hi "+r.URL.Path) }))

	client, base := Dialer(URLPrefix+path, nil)
	resp, err := client.Get(base + "/x")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "hi /x" {
		t.Fatalf("%q", body)
	}
	ln.Close()
	if _, err := os.Stat(path); err == nil {
		t.Error("socket file left behind")
	}
}

func TestSocketsAreUniquePerListener(t *testing.T) {
	t.Setenv("TMPDIR", shortTmp(t))
	l1, p1, err := ListenSocket()
	if err != nil {
		t.Fatal(err)
	}
	defer l1.Close()
	l2, p2, err := ListenSocket()
	if err != nil {
		t.Fatal(err)
	}
	defer l2.Close()
	if p1 == p2 {
		t.Fatal("two listeners share a path")
	}
}

func TestSocketDirIsPrivateAndLooseModesAreTightened(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no modes on Windows")
	}
	tmp := shortTmp(t)
	t.Setenv("TMPDIR", tmp)
	dir := SocketDir()
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	os.Chmod(dir, 0o755)
	ln, _, err := ListenSocket()
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if fi, _ := os.Stat(dir); fi.Mode().Perm() != 0o700 {
		t.Fatalf("directory left at %v", fi.Mode().Perm())
	}
}

func TestSocketDirThatIsASymlinkOrAFileIsRefused(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	tmp := shortTmp(t)
	t.Setenv("TMPDIR", tmp)
	dir := SocketDir()
	elsewhere := filepath.Join(tmp, "elsewhere")
	os.Mkdir(elsewhere, 0o700)
	if err := os.Symlink(elsewhere, dir); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ListenSocket(); err == nil {
		t.Fatal("a planted symlink was accepted")
	}
	os.Remove(dir)
	os.WriteFile(dir, nil, 0o600)
	if _, _, err := ListenSocket(); err == nil {
		t.Fatal("a file in place of the directory was accepted")
	}
}

func TestDialerUsesPlainHTTPForOtherAddresses(t *testing.T) {
	c, base := Dialer("http://127.0.0.1:9/", nil)
	if c == nil || base != "http://127.0.0.1:9" || strings.Contains(base, "unix") {
		t.Fatalf("%v %q", c, base)
	}
}

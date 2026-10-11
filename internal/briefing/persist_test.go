package briefing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dkage/dungeon-master/internal/testutil"
)

func TestSavePreviewIsPrivateAndCannotEscapeStateDirectory(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	id, path, err := SavePreview(state, "c1234abcd", 42, "opus", "private brief")
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 32 || !strings.HasPrefix(path, filepath.Join(state, "briefings", "c1234abcd")) {
		t.Fatalf("unexpected preview location: %q %q", id, path)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "private brief" {
		t.Fatalf("saved content %q, err %v", data, err)
	}
	info, err := os.Stat(path)
	if err != nil || !testutil.PrivateMode(path, info, 0o600) {
		t.Fatalf("file mode %v, err %v", info.Mode().Perm(), err)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil || !testutil.PrivateMode(filepath.Dir(path), dirInfo, 0o700) {
		t.Fatalf("directory mode %v, err %v", dirInfo.Mode().Perm(), err)
	}
	resolved, err := PreviewPath(state, "c1234abcd", 42, "opus", id)
	if err != nil || resolved != path {
		t.Fatalf("resolved %q, err %v", resolved, err)
	}
	for _, args := range [][5]string{
		{state, "../../etc", "42", "opus", id},
		{state, "c1234abcd", "42", "../opus", id},
		{state, "c1234abcd", "42", "opus", "../../etc"},
	} {
		if _, err := PreviewPath(args[0], args[1], 42, args[3], args[4]); err == nil {
			t.Fatalf("accepted unsafe location: %v", args)
		}
	}
}

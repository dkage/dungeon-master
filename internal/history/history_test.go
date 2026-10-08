package history

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func rec(id, campaign string) Record {
	return Record{ID: id, Campaign: campaign, Kind: "hero", Hero: "opus", Title: "Opus", Dir: "/p", Created: time.Now().UTC()}
}

func TestAddFinishRemove(t *testing.T) {
	s, _ := Open("")
	_ = s.Add(rec("a", "c1"))
	_ = s.Add(rec("b", "c1"))
	if err := s.Finish("a", 3); err != nil {
		t.Fatal(err)
	}
	a, ok := s.Get("a")
	if !ok || a.Ended == nil || a.ExitCode != 3 || a.Lost {
		t.Fatalf("a: %+v", a)
	}
	if b, _ := s.Get("b"); b.Ended != nil {
		t.Fatal("b should still be running")
	}
	if err := s.Finish("ghost", 0); err != nil {
		t.Fatalf("finishing an unknown id must be harmless: %v", err)
	}
	if ok, _ := s.Remove("a"); !ok {
		t.Fatal("remove failed")
	}
	if ok, _ := s.Remove("a"); ok {
		t.Fatal("removed twice")
	}
	if got := s.List(); len(got) != 1 || got[0].ID != "b" {
		t.Fatalf("list: %+v", got)
	}
}

func TestRestartMarksRunningRecordsLost(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	s, _ := Open(path)
	_ = s.Add(rec("done", "c1"))
	_ = s.Finish("done", 0)
	_ = s.Add(rec("running", "c1"))

	again, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	done, _ := again.Get("done")
	if done.Lost || done.ExitCode != 0 {
		t.Fatalf("finished record changed: %+v", done)
	}
	run, _ := again.Get("running")
	if !run.Lost || run.Ended == nil || run.ExitCode != -1 {
		t.Fatalf("running record not marked lost: %+v", run)
	}
	// The correction was persisted: a third open sees the same.
	third, _ := Open(path)
	if r, _ := third.Get("running"); !r.Lost {
		t.Fatal("lost mark not persisted")
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", fi.Mode().Perm())
	}
}

func TestRemoveCampaign(t *testing.T) {
	s, _ := Open("")
	_ = s.Add(rec("a", "c1"))
	_ = s.Add(rec("b", "c2"))
	_ = s.Add(rec("c", "c1"))
	if err := s.RemoveCampaign("c1"); err != nil {
		t.Fatal(err)
	}
	if got := s.List(); len(got) != 1 || got[0].ID != "b" {
		t.Fatalf("list: %+v", got)
	}
}

func TestTrimDropsOldestFinishedOnly(t *testing.T) {
	s, _ := Open("")
	_ = s.Add(rec("keep-running", "c")) // oldest, but still running
	for i := 0; i < MaxRecords+5; i++ {
		id := fmt.Sprintf("r%d", i)
		_ = s.Add(rec(id, "c"))
		_ = s.Finish(id, 0)
	}
	got := s.List()
	if len(got) != MaxRecords {
		t.Fatalf("len %d, want %d", len(got), MaxRecords)
	}
	if _, ok := s.Get("keep-running"); !ok {
		t.Fatal("a running record was trimmed")
	}
	if _, ok := s.Get("r0"); ok {
		t.Fatal("the oldest finished record should have been dropped")
	}
	if _, ok := s.Get(fmt.Sprintf("r%d", MaxRecords+4)); !ok {
		t.Fatal("newest record missing")
	}
}

func TestOpenRejectsCorruptFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sessions.json")
	_ = os.WriteFile(p, []byte("{nope"), 0o600)
	if _, err := Open(p); err == nil {
		t.Fatal("corrupt file accepted")
	}
}

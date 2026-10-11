package gatekeeper

import (
	"strings"
	"testing"
)

func TestWindowsMixedCaseEnvironment(t *testing.T) {
	env := SessionEnv{BinDir: "C:\\gk", Token: "new"}.Apply([]string{"Path=C:\\real", "dungeon_gate_token=old"})
	paths := 0
	for _, kv := range env {
		if strings.HasPrefix(strings.ToUpper(kv), "PATH=") {
			paths++
			if kv != "PATH=C:\\gk;C:\\real" {
				t.Fatal(kv)
			}
		}
		if strings.Contains(kv, "=old") {
			t.Fatal("old token leaked")
		}
	}
	if paths != 1 {
		t.Fatal("duplicate PATH")
	}
	child := childEnv(ShimOptions{Getenv: func(k string) string {
		if k == EnvGateBin {
			return "c:\\GK"
		}
		return ""
	}, Environ: []string{"Path=C:\\gk;C:\\real"}})
	if len(child) != 1 || child[0] != "PATH=C:\\real" {
		t.Fatal(child)
	}
}

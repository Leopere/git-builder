package runlog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppend(t *testing.T) {
	t.Run("empty path no-op", func(t *testing.T) {
		if err := Append("", Event{Phase: "start"}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("writes JSON line", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "nested", "runs.log")
		ev := Event{
			Time:       "2026-03-20T12:00:00.123456789Z",
			RepoURL:    "https://github.com/o/r.git",
			Commit:     "abc",
			ScriptKind: "in-repo",
			ScriptPath: "/tmp/r/.git-builder.sh",
			Phase:      "start",
		}
		if err := Append(p, ev); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if len(b) < 10 || b[len(b)-1] != '\n' {
			t.Fatalf("expected newline-terminated JSON, got %q", b)
		}
	})
}

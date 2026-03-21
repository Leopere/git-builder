// Package runlog appends JSON Lines run audit events (optional file, mutex-safe).
package runlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

var mu sync.Mutex

// Event is one JSON object per line (RFC3339Nano times).
type Event struct {
	Time       string `json:"time"`
	RepoURL    string `json:"repo_url"`
	Commit     string `json:"commit"`
	ScriptKind string `json:"script_kind"`
	ScriptPath string `json:"script_path"`
	Phase      string `json:"phase"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
}

// Append writes one JSON line to path. Empty path is a no-op.
func Append(path string, ev Event) error {
	if path == "" {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = f.Write(b)
	return err
}

package svc

import (
	"os"
	"path/filepath"
	"runtime"
)

func RunDir() string {
	if runtime.GOOS == "darwin" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".local", "var", "run", "git-builder")
		}
	}
	execPath, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(execPath)
}

func PidPath() string   { return filepath.Join(RunDir(), "git-builder.pid") }
func StatePath() string { return filepath.Join(RunDir(), "git-builder.state") }

package run

import (
	"bufio"
	"context"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

const ScriptName = ".git-builder.sh"

func RunIfPresent(ctx context.Context, repoDir, overridePath string) error {
	scriptPath, err := chooseScriptPath(repoDir, overridePath)
	if err != nil || scriptPath == "" {
		return err
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", scriptPath)
	cmd.Dir = repoDir
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	go logPipe("[git-builder stdout]", stdout)
	go logPipe("[git-builder stderr]", stderr)

	return cmd.Wait()
}

func chooseScriptPath(repoDir, overridePath string) (string, error) {
	if overridePath != "" {
		if _, err := os.Stat(overridePath); err == nil {
			return overridePath, nil
		}
	}
	inRepo := filepath.Join(repoDir, ScriptName)
	if _, err := os.Stat(inRepo); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return inRepo, nil
}

func logPipe(prefix string, r io.Reader) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		log.Printf("%s %s", prefix, sc.Text())
	}
}

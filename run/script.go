package run

import (
	"bufio"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

const ScriptName = ".git-builder.sh"

func RunIfPresent(repoDir string) error {
	scriptPath := filepath.Join(repoDir, ScriptName)
	if _, err := os.Stat(scriptPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	cmd := exec.Command("sh", "-c", scriptPath)
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

func logPipe(prefix string, r io.Reader) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		log.Printf("%s %s", prefix, sc.Text())
	}
}

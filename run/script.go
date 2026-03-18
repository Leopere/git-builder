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

func RunIfPresent(ctx context.Context, repoDir, overridePath string, extraEnv map[string]string) error {
	scriptPath, err := chooseScriptPath(repoDir, overridePath)
	if err != nil || scriptPath == "" {
		return err
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", scriptPath)
	cmd.Dir = repoDir
	cmd.Env = appendEnviron(os.Environ(), extraEnv)

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

// RunIfPresentWithStdio runs the script if present. If stdout != nil, script stdout/stderr
// are connected directly to the given writers (passthrough). Otherwise behaves like RunIfPresent (pipes + log).
func RunIfPresentWithStdio(ctx context.Context, repoDir, overridePath string, extraEnv map[string]string, stdout, stderr io.Writer) error {
	scriptPath, err := chooseScriptPath(repoDir, overridePath)
	if err != nil || scriptPath == "" {
		return err
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", scriptPath)
	cmd.Dir = repoDir
	cmd.Env = appendEnviron(os.Environ(), extraEnv)

	if stdout != nil && stderr != nil {
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		return cmd.Run()
	}

	pipeOut, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	pipeErr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go logPipe("[git-builder stdout]", pipeOut)
	go logPipe("[git-builder stderr]", pipeErr)
	return cmd.Wait()
}

func appendEnviron(base []string, extra map[string]string) []string {
	if len(extra) == 0 {
		return base
	}
	env := make([]string, len(base), len(base)+len(extra))
	copy(env, base)
	for k, v := range extra {
		env = append(env, k+"="+v)
	}
	return env
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

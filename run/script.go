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

// Script kind labels for ResolveScript and audit logs.
const (
	ScriptKindOverride = "override"
	ScriptKindInRepo   = "in-repo"
)

// ResolveScript returns the script path to run and its kind, or ("", "", nil) if none.
func ResolveScript(repoDir, overridePath string) (scriptPath string, kind string, err error) {
	if overridePath != "" {
		if _, err := os.Stat(overridePath); err == nil {
			return overridePath, ScriptKindOverride, nil
		}
	}
	inRepo := filepath.Join(repoDir, ScriptName)
	if _, err := os.Stat(inRepo); err != nil {
		if os.IsNotExist(err) {
			return "", "", nil
		}
		return "", "", err
	}
	return inRepo, ScriptKindInRepo, nil
}

// RunResolved runs sh -c scriptPath with working directory repoDir (stdout/stderr piped to log).
func RunResolved(ctx context.Context, repoDir, scriptPath string, extraEnv map[string]string) error {
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

// RunResolvedWithStdio runs the resolved script. If both stdout and stderr are non-nil, output goes there; otherwise pipes to log.
func RunResolvedWithStdio(ctx context.Context, repoDir, scriptPath string, extraEnv map[string]string, stdout, stderr io.Writer) error {
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

// RunIfPresent runs the script if present. Returns (true, nil) if script ran successfully,
// (true, err) if script ran and failed, (false, nil) if no script present, (false, err) on path error.
func RunIfPresent(ctx context.Context, repoDir, overridePath string, extraEnv map[string]string) (ran bool, err error) {
	scriptPath, _, err := ResolveScript(repoDir, overridePath)
	if err != nil {
		return false, err
	}
	if scriptPath == "" {
		return false, nil
	}
	return true, RunResolved(ctx, repoDir, scriptPath, extraEnv)
}

// RunIfPresentWithStdio runs the script if present. Returns (true, nil) if script ran successfully,
// (true, err) if script ran and failed, (false, nil) if no script present. If stdout != nil, script
// stdout/stderr are connected directly to the given writers (passthrough).
func RunIfPresentWithStdio(ctx context.Context, repoDir, overridePath string, extraEnv map[string]string, stdout, stderr io.Writer) (ran bool, err error) {
	scriptPath, _, err := ResolveScript(repoDir, overridePath)
	if err != nil {
		return false, err
	}
	if scriptPath == "" {
		return false, nil
	}
	return true, RunResolvedWithStdio(ctx, repoDir, scriptPath, extraEnv, stdout, stderr)
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

func logPipe(prefix string, r io.Reader) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		log.Printf("%s %s", prefix, sc.Text())
	}
}

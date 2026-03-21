package main

import (
	"context"
	"io"
	"log"
	"strings"
	"time"

	"git-builder/config"
	"git-builder/run"
	"git-builder/runlog"
)

// runScriptWithAudit resolves and runs the build script, logging start/end to stdout and optional run_log_path (JSON Lines).
// passthroughOut and passthroughErr both non-nil selects trigger-style stdio; otherwise output is piped to the logger.
func runScriptWithAudit(ctx context.Context, cfg *config.Config, repoURL, repoDir, fullHash, overridePath string, scriptEnv map[string]string, passthroughOut, passthroughErr io.Writer) (ran bool, err error) {
	path, kind, err := run.ResolveScript(repoDir, overridePath)
	if err != nil {
		return false, err
	}
	if path == "" {
		return false, nil
	}
	logPath := strings.TrimSpace(cfg.RunLogPath)
	t0 := time.Now()
	revDisplay := shortRevOrQuery(repoDir)
	log.Printf("script start %s %s path=%s kind=%s", repoURL, revDisplay, path, kind)
	appendRunLog(logPath, runlog.Event{
		Time:       t0.UTC().Format(time.RFC3339Nano),
		RepoURL:    repoURL,
		Commit:     fullHash,
		ScriptKind: kind,
		ScriptPath: path,
		Phase:      "start",
	})
	var runErr error
	if passthroughOut != nil && passthroughErr != nil {
		runErr = run.RunResolvedWithStdio(ctx, repoDir, path, scriptEnv, passthroughOut, passthroughErr)
	} else {
		runErr = run.RunResolved(ctx, repoDir, path, scriptEnv)
	}
	dur := time.Since(t0)
	end := runlog.Event{
		Time:       time.Now().UTC().Format(time.RFC3339Nano),
		RepoURL:    repoURL,
		Commit:     fullHash,
		ScriptKind: kind,
		ScriptPath: path,
		Phase:      "success",
		DurationMs: dur.Milliseconds(),
	}
	if runErr != nil {
		end.Phase = "failure"
		end.Error = runErr.Error()
	}
	appendRunLog(logPath, end)
	return true, runErr
}

func appendRunLog(path string, ev runlog.Event) {
	if err := runlog.Append(path, ev); err != nil {
		log.Printf("run log: append failed: %v", err)
	}
}

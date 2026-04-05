package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"git-builder/config"
	"git-builder/gitops"
	"git-builder/svc"
)

const attributionURL = "https://colinknapp.com?utm_source=cli&utm_medium=banner&utm_campaign=git-builder"

// lineFlushWriter flushes after each write that contains a newline so output appears in real time (e.g. over SSH).
type lineFlushWriter struct{ w io.Writer }

func (f *lineFlushWriter) Write(p []byte) (n int, err error) {
	n, err = f.w.Write(p)
	if err == nil && bytes.Contains(p, []byte{'\n'}) {
		if file, ok := f.w.(*os.File); ok {
			_ = file.Sync()
		}
	}
	return n, err
}

func main() {
	log.SetOutput(&lineFlushWriter{w: os.Stdout})
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	doInstall := flag.Bool("install", false, "install and start service (use 'go run . -install' for install-from-source)")
	doUninstall := flag.Bool("uninstall", false, "remove service and binary")
	listJobs := flag.Bool("listjobs", false, "print current job (or idle) from running daemon")
	killJobs := flag.Bool("killjobs", false, "signal daemon to cancel current job")
	runOnce := flag.Bool("run-once", false, "run one poll cycle then exit (for on-demand or testing)")
	triggerURL := flag.String("trigger", "", "sync and run script for this repo URL once then exit")
	flag.Parse()

	if *doInstall {
		if err := svc.Install(); err != nil {
			log.Fatalf("install: %v", err)
		}
		return
	}
	if *doUninstall {
		if err := svc.Uninstall(); err != nil {
			log.Fatalf("uninstall: %v", err)
		}
		return
	}
	if *listJobs {
		if err := svc.ListJobs(); err != nil {
			log.Fatalf("listjobs: %v", err)
		}
		return
	}
	if *killJobs {
		if err := svc.KillJobs(); err != nil {
			log.Fatalf("killjobs: %v", err)
		}
		return
	}

	cfgPath := config.ResolvePath("")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	if *runOnce {
		sem := make(chan struct{}, cfg.MaxConcurrent)
		var wg sync.WaitGroup
		for _, repo := range cfg.Repos {
			if repo.URL == "" {
				continue
			}
			repo := repo
			display := repoDisplay(repo)
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				log.Printf("sync %s", display)
				localPath, updated, err := gitops.Sync(cfg, repo)
				if err != nil {
					log.Printf("sync failed %s: %v", display, err)
					return
				}
				logSyncOutcome("", display, localPath, updated)
				if !updated {
					return
				}
				fullHash, err := gitops.FullRevision(localPath)
				if err != nil {
					log.Printf("sync succeeded but read head %s: %v", display, err)
					return
				}
				if gitops.IsDeployed(cfg.Workdir, repo, fullHash) {
					log.Printf("sync skipped %s (already deployed at this commit) %s", display, shortRevOrQuery(localPath))
					return
				}
				defer removeRepoWorkdir(localPath)
				overridePath := ""
				if d := cfg.OverrideScriptDir(); d != "" {
					overridePath = filepath.Join(d, gitops.OverrideScriptBasename(repo.URL)+".sh")
				}
				scriptEnv := scriptEnvFromConfig(cfg)
				if token := scriptEnv["GHCR_TOKEN"]; token != "" {
					if err := dockerLoginGHCR(context.Background(), token, strings.TrimSpace(cfg.GhcrUser)); err != nil {
						log.Printf("ghcr login failed %s: %v", display, err)
						return
					}
				}
				ran, err := runScriptWithAudit(context.Background(), cfg, repo.URL, localPath, fullHash, overridePath, scriptEnv, nil, nil)
				if err != nil {
					log.Printf("script failed %s: %v", display, err)
					return
				}
				if ran {
					logScriptSuccess(display, localPath)
				} else {
					log.Printf("no script to run %s", display)
				}
				if err := gitops.SetDeployed(cfg.Workdir, repo, fullHash); err != nil {
					log.Printf("warning: save deploy state %s: %v", display, err)
				}
			}()
		}
		wg.Wait()
		return
	}

	if *triggerURL != "" {
		matches := matchingRepos(cfg.Repos, *triggerURL)
		if len(matches) == 0 {
			log.Fatalf("trigger: repo %q not in config", *triggerURL)
		}
		for _, repo := range matches {
			display := repoDisplay(repo)
			log.Printf("trigger: syncing %s", display)
			localPath, updated, err := gitops.Sync(cfg, repo)
			if err != nil {
				log.Fatalf("trigger: sync failed %s: %v", display, err)
			}
			logSyncOutcome("trigger: ", display, localPath, updated)
			fullHash, err := gitops.FullRevision(localPath)
			if err != nil {
				log.Fatalf("trigger: read head %s: %v", display, err)
			}
			defer removeRepoWorkdir(localPath)
			overridePath := ""
			if d := cfg.OverrideScriptDir(); d != "" {
				overridePath = filepath.Join(d, gitops.OverrideScriptBasename(repo.URL)+".sh")
			}
			scriptEnv := scriptEnvFromConfig(cfg)
			if token := scriptEnv["GHCR_TOKEN"]; token != "" {
				if err := dockerLoginGHCR(context.Background(), token, strings.TrimSpace(cfg.GhcrUser)); err != nil {
					log.Fatalf("trigger: ghcr login failed %s: %v", display, err)
				}
				log.Printf("trigger: logged into ghcr.io")
			}
			log.Printf("trigger: passing %d script env vars", len(scriptEnv))
			ran, err := runScriptWithAudit(context.Background(), cfg, repo.URL, localPath, fullHash, overridePath, scriptEnv, os.Stdout, os.Stderr)
			if err != nil {
				log.Fatalf("trigger: script failed %s: %v", display, err)
			}
			if ran {
				log.Printf("trigger: script completed successfully, done %s %s", display, shortRevOrQuery(localPath))
			} else {
				log.Printf("trigger: no script to run, done %s", display)
			}
			if err := gitops.SetDeployed(cfg.Workdir, repo, fullHash); err != nil {
				log.Printf("trigger: warning: save deploy state %s: %v", display, err)
			}
		}
		return
	}

	reloadCh := config.Watch(cfgPath)

	log.Printf("git-builder by Colin Knapp — %s", attributionURL)

	if err := svc.WritePid(os.Getpid()); err != nil {
		log.Printf("warning: could not write pid file: %v", err)
	}
	defer svc.RemovePid()

	var jobMu sync.Mutex
	activeJobs := make(map[string]context.CancelFunc)
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, svc.SIGUSR1, svc.SIGUSR2)

	tick := time.NewTicker(time.Duration(cfg.PollIntervalSeconds) * time.Second)
	defer tick.Stop()

	doPoll := func() {
		sem := make(chan struct{}, cfg.MaxConcurrent)
		var wg sync.WaitGroup
		for _, repo := range cfg.Repos {
			if repo.URL == "" {
				continue
			}
			repo := repo
			display := repoDisplay(repo)
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				log.Printf("sync %s", display)
				localPath, updated, err := gitops.Sync(cfg, repo)
				if err != nil {
					log.Printf("sync failed %s: %v", display, err)
					return
				}
				logSyncOutcome("", display, localPath, updated)
				if !updated {
					return
				}
				fullHash, err := gitops.FullRevision(localPath)
				if err != nil {
					log.Printf("sync succeeded but read head %s: %v", display, err)
					return
				}
				if gitops.IsDeployed(cfg.Workdir, repo, fullHash) {
					log.Printf("sync skipped %s (already deployed at this commit) %s", display, shortRevOrQuery(localPath))
					return
				}
				defer removeRepoWorkdir(localPath)
				overridePath := ""
				if d := cfg.OverrideScriptDir(); d != "" {
					overridePath = filepath.Join(d, gitops.OverrideScriptBasename(repo.URL)+".sh")
				}
				scriptEnv := scriptEnvFromConfig(cfg)
				ctx, cancel := context.WithCancel(context.Background())
				if token := scriptEnv["GHCR_TOKEN"]; token != "" {
					if err := dockerLoginGHCR(ctx, token, strings.TrimSpace(cfg.GhcrUser)); err != nil {
						log.Printf("ghcr login failed %s: %v", display, err)
						cancel()
						return
					}
				}
				jobMu.Lock()
				activeJobs[display] = cancel
				state := strings.Join(activeJobURLs(activeJobs), ",")
				jobMu.Unlock()
				_ = svc.WriteState(state)
				ran, err := runScriptWithAudit(ctx, cfg, repo.URL, localPath, fullHash, overridePath, scriptEnv, nil, nil)
				if err != nil {
					log.Printf("script failed %s: %v", display, err)
				} else if ran {
					logScriptSuccess(display, localPath)
				} else {
					log.Printf("no script to run %s", display)
				}
				if err == nil {
					if err := gitops.SetDeployed(cfg.Workdir, repo, fullHash); err != nil {
						log.Printf("warning: save deploy state %s: %v", display, err)
					}
				}
				jobMu.Lock()
				delete(activeJobs, display)
				state = "idle"
				if len(activeJobs) > 0 {
					state = strings.Join(activeJobURLs(activeJobs), ",")
				}
				jobMu.Unlock()
				cancel()
				_ = svc.WriteState(state)
			}()
		}
		wg.Wait()
	}

	_ = svc.WriteState("idle")
	doPoll()
	for {
		select {
		case sig := <-sigCh:
			if sig == svc.SIGUSR1 {
				jobMu.Lock()
				state := "idle"
				if len(activeJobs) > 0 {
					state = strings.Join(activeJobURLs(activeJobs), ",")
				}
				jobMu.Unlock()
				_ = svc.WriteState(state)
				continue
			}
			if sig == svc.SIGUSR2 {
				jobMu.Lock()
				for _, c := range activeJobs {
					c()
				}
				jobMu.Unlock()
				continue
			}
			log.Print("shutting down")
			return
		case newCfg := <-reloadCh:
			if newCfg.PollIntervalSeconds != cfg.PollIntervalSeconds {
				tick.Stop()
				tick = time.NewTicker(time.Duration(newCfg.PollIntervalSeconds) * time.Second)
			}
			cfg = newCfg
			log.Print("config reloaded")
		case <-tick.C:
			doPoll()
		}
	}
}

func activeJobURLs(jobs map[string]context.CancelFunc) []string {
	urls := make([]string, 0, len(jobs))
	for u := range jobs {
		urls = append(urls, u)
	}
	return urls
}

const defaultGhcrUser = "Leopere"

// dockerLoginGHCR runs `docker login ghcr.io` so the daemon can pull from GHCR.
// user may be empty; then defaultGhcrUser is used.
func dockerLoginGHCR(ctx context.Context, token, user string) error {
	if user == "" {
		user = defaultGhcrUser
	}
	cmd := exec.CommandContext(ctx, "docker", "login", "ghcr.io", "-u", user, "--password-stdin")
	cmd.Stdin = strings.NewReader(token)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker login ghcr.io: %w\n%s", err, out)
	}
	return nil
}

func scriptEnvFromConfig(cfg *config.Config) map[string]string {
	out := make(map[string]string)
	for k, v := range cfg.ScriptEnv {
		if s := strings.TrimSpace(v); s != "" {
			out[k] = s
		}
	}
	for _, token := range []string{strings.TrimSpace(cfg.GhcrToken), strings.TrimSpace(cfg.GhcrTokenAlt)} {
		if token != "" && out["GHCR_TOKEN"] == "" {
			out["GHCR_TOKEN"] = token
			break
		}
	}
	if out["GHCR_TOKEN"] == "" {
		// Fallback for repos that only know about GHCR_TOKEN (e.g. bert-ner),
		// while our config may only provide github_token.
		if t := strings.TrimSpace(cfg.TokenFromConfig); t != "" {
			out["GHCR_TOKEN"] = t
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

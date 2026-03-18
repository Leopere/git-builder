package main

import (
	"bytes"
	"context"
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"git-builder/config"
	"git-builder/gitops"
	"git-builder/run"
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
	log.SetFlags(log.LstdFlags)

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
		for _, r := range cfg.Repos {
			if r.URL == "" {
				continue
			}
			url := r.URL
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				log.Printf("sync %s", url)
				localPath, updated, err := gitops.Sync(cfg, url)
				if err != nil {
					log.Printf("sync failed %s: %v", url, err)
					return
				}
				if updated {
					log.Printf("sync succeeded %s (repo updated)", url)
				} else {
					log.Printf("sync succeeded %s (already up to date)", url)
				}
				if !updated {
					return
				}
				overridePath := ""
				if d := cfg.OverrideScriptDir(); d != "" {
					overridePath = filepath.Join(d, gitops.OverrideScriptBasename(url)+".sh")
				}
				scriptEnv := scriptEnvFromConfig(cfg)
				ran, err := run.RunIfPresent(context.Background(), localPath, overridePath, scriptEnv)
				if err != nil {
					log.Printf("script failed %s: %v", url, err)
				} else if ran {
					log.Printf("script completed successfully %s", url)
				} else {
					log.Printf("no script to run %s", url)
				}
			}()
		}
		wg.Wait()
		return
	}

	if *triggerURL != "" {
		var found bool
		for _, r := range cfg.Repos {
			if r.URL == *triggerURL {
				found = true
				break
			}
		}
		if !found {
			log.Fatalf("trigger: repo %q not in config", *triggerURL)
		}
		log.Printf("trigger: syncing %s", *triggerURL)
		localPath, updated, err := gitops.Sync(cfg, *triggerURL)
		if err != nil {
			log.Fatalf("trigger: sync failed %s: %v", *triggerURL, err)
		}
		if updated {
			log.Printf("trigger: sync succeeded %s (repo updated)", *triggerURL)
		} else {
			log.Printf("trigger: sync succeeded %s (already up to date)", *triggerURL)
		}
		overridePath := ""
		if d := cfg.OverrideScriptDir(); d != "" {
			overridePath = filepath.Join(d, gitops.OverrideScriptBasename(*triggerURL)+".sh")
		}
		log.Printf("trigger: running script (repo=%s)", localPath)
		scriptEnv := scriptEnvFromConfig(cfg)
		log.Printf("trigger: passing %d script env vars", len(scriptEnv))
		ran, err := run.RunIfPresentWithStdio(context.Background(), localPath, overridePath, scriptEnv, os.Stdout, os.Stderr)
		if err != nil {
			log.Fatalf("trigger: script failed %s: %v", *triggerURL, err)
		}
		if ran {
			log.Printf("trigger: script completed successfully, done %s", *triggerURL)
		} else {
			log.Printf("trigger: no script to run, done %s", *triggerURL)
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
		for _, r := range cfg.Repos {
			if r.URL == "" {
				continue
			}
			url := r.URL
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				log.Printf("sync %s", url)
				localPath, updated, err := gitops.Sync(cfg, url)
				if err != nil {
					log.Printf("sync failed %s: %v", url, err)
					return
				}
				if updated {
					log.Printf("sync succeeded %s (repo updated)", url)
				} else {
					log.Printf("sync succeeded %s (already up to date)", url)
				}
				if !updated {
					return
				}
				overridePath := ""
				if d := cfg.OverrideScriptDir(); d != "" {
					overridePath = filepath.Join(d, gitops.OverrideScriptBasename(url)+".sh")
				}
				scriptEnv := scriptEnvFromConfig(cfg)
				ctx, cancel := context.WithCancel(context.Background())
				jobMu.Lock()
				activeJobs[url] = cancel
				state := strings.Join(activeJobURLs(activeJobs), ",")
				jobMu.Unlock()
				_ = svc.WriteState(state)
				ran, err := run.RunIfPresent(ctx, localPath, overridePath, scriptEnv)
				if err != nil {
					log.Printf("script failed %s: %v", url, err)
				} else if ran {
					log.Printf("script completed successfully %s", url)
				} else {
					log.Printf("no script to run %s", url)
				}
				jobMu.Lock()
				delete(activeJobs, url)
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
	if len(out) == 0 {
		return nil
	}
	return out
}

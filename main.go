package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
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

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags)

	doInstall := flag.Bool("install", false, "install and start service (use 'go run . -install' for install-from-source)")
	doUninstall := flag.Bool("uninstall", false, "remove service and binary")
	listJobs := flag.Bool("listjobs", false, "print current job (or idle) from running daemon")
	killJobs := flag.Bool("killjobs", false, "signal daemon to cancel current job")
	runOnce := flag.Bool("run-once", false, "run one poll cycle then exit (for on-demand or testing)")
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
				if !updated {
					return
				}
				if err := run.RunIfPresent(context.Background(), localPath); err != nil {
					log.Printf("script failed %s: %v", url, err)
				}
			}()
		}
		wg.Wait()
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
				if !updated {
					return
				}
				ctx, cancel := context.WithCancel(context.Background())
				jobMu.Lock()
				activeJobs[url] = cancel
				state := strings.Join(activeJobURLs(activeJobs), ",")
				jobMu.Unlock()
				_ = svc.WriteState(state)
				if err := run.RunIfPresent(ctx, localPath); err != nil {
					log.Printf("script failed %s: %v", url, err)
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

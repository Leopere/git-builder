package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"git-builder/config"
	"git-builder/gitops"
	"git-builder/run"
	"git-builder/svc"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags)

	doInstall := flag.Bool("install", false, "install and start service (use 'go run . -install' for install-from-source)")
	doUninstall := flag.Bool("uninstall", false, "remove service and binary")
	listJobs := flag.Bool("listjobs", false, "print current job (or idle) from running daemon")
	killJobs := flag.Bool("killjobs", false, "signal daemon to cancel current job")
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
	reloadCh := config.Watch(cfgPath)

	if err := svc.WritePid(os.Getpid()); err != nil {
		log.Printf("warning: could not write pid file: %v", err)
	}
	defer svc.RemovePid()

	var currentJob string
	var cancel context.CancelFunc
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, svc.SIGUSR1, svc.SIGUSR2)

	tick := time.NewTicker(time.Duration(cfg.PollIntervalSeconds) * time.Second)
	defer tick.Stop()

	doPoll := func() {
		for _, r := range cfg.Repos {
			if r.URL == "" {
				continue
			}
			log.Printf("sync %s", r.URL)
			localPath, err := gitops.Sync(cfg, r.URL)
			if err != nil {
				log.Printf("sync failed %s: %v", r.URL, err)
				continue
			}
			ctx, c := context.WithCancel(context.Background())
			cancel = c
			currentJob = r.URL
			_ = svc.WriteState(currentJob)
			if err := run.RunIfPresent(ctx, localPath); err != nil {
				log.Printf("script failed %s: %v", r.URL, err)
			}
			cancel = nil
			currentJob = ""
			_ = svc.WriteState("idle")
		}
	}

	_ = svc.WriteState("idle")
	doPoll()
	for {
		select {
		case sig := <-sigCh:
			if sig == svc.SIGUSR1 {
				if currentJob != "" {
					_ = svc.WriteState(currentJob)
				} else {
					_ = svc.WriteState("idle")
				}
				continue
			}
			if sig == svc.SIGUSR2 && cancel != nil {
				cancel()
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

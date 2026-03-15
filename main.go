package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"git-builder/config"
	"git-builder/gitops"
	"git-builder/run"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags)

	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := make(chan os.Signal, 1)
	signal.Notify(ctx, syscall.SIGTERM, syscall.SIGINT)

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
			if err := run.RunIfPresent(localPath); err != nil {
				log.Printf("script failed %s: %v", r.URL, err)
			}
		}
	}

	doPoll()
	for {
		select {
		case <-ctx:
			log.Print("shutting down")
			return
		case <-tick.C:
			doPoll()
		}
	}
}

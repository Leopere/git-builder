package main

import (
	"log"
	"os"

	"git-builder/gitops"
)

func shortRevOrQuery(localPath string) string {
	if rev, err := gitops.ShortRevision(localPath); err == nil {
		return rev
	}
	return "?"
}

func logSyncOutcome(prefix, url, localPath string, updated bool) {
	rev := shortRevOrQuery(localPath)
	if updated {
		log.Printf("%ssync succeeded %s (repo updated) %s", prefix, url, rev)
	} else {
		log.Printf("%ssync succeeded %s (already up to date) %s", prefix, url, rev)
	}
}

func logScriptSuccess(url, localPath string) {
	log.Printf("script completed successfully %s %s", url, shortRevOrQuery(localPath))
}

// removeRepoWorkdir deletes the clone under workdir so only journal logs and deploy state remain on disk.
func removeRepoWorkdir(path string) {
	if err := os.RemoveAll(path); err != nil {
		log.Printf("warning: remove repo working tree %s: %v", path, err)
	}
}

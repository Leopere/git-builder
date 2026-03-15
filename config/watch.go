package config

import (
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

const reloadDebounce = 300 * time.Millisecond

// Watch watches the config file and sends the new config on the returned
// channel whenever the file is written and loads successfully.
// The caller receives the initial load from Load(); this only sends on changes.
func Watch(path string) <-chan *Config {
	path = ResolvePath(path)
	ch := make(chan *Config, 1)
	dir := filepath.Dir(path)

	absPath, _ := filepath.Abs(path)
	go func() {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return
		}
		defer watcher.Close()
		if err := watcher.Add(dir); err != nil {
			return
		}
		var timer *time.Timer
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if filepath.Clean(event.Name) != absPath {
					continue
				}
				if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
					continue
				}
				if timer != nil {
					timer.Stop()
				}
				timer = time.AfterFunc(reloadDebounce, func() {
					newCfg, err := Load(path)
					if err != nil {
						return
					}
					select {
					case ch <- newCfg:
					default:
						// consumer not ready; drop so we don't block watcher
					}
				})
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			}
		}
	}()
	return ch
}

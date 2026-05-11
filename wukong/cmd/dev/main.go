package main

import (
	"context"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

const debounceWindow = 250 * time.Millisecond

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("create watcher failed: %v", err)
	}
	defer watcher.Close()

	if err := watchGoDirs(watcher, "."); err != nil {
		log.Fatalf("register watch dirs failed: %v", err)
	}

	log.Println("[dev] watching .go files, restart on change...")

	var (
		mu  sync.Mutex
		cmd *exec.Cmd
	)

	startServer := func() error {
		mu.Lock()
		defer mu.Unlock()

		if cmd != nil {
			_ = terminateProcess(cmd)
		}

		next := exec.Command("go", "run", "./cmd/server")
		next.Stdout = os.Stdout
		next.Stderr = os.Stderr
		next.Stdin = os.Stdin
		next.Env = os.Environ()

		if err := next.Start(); err != nil {
			return err
		}
		cmd = next
		log.Printf("[dev] server started pid=%d", next.Process.Pid)
		return nil
	}

	scheduleRestart := newDebouncer(debounceWindow, func() {
		if err := startServer(); err != nil {
			log.Printf("[dev] restart failed: %v", err)
		}
	})

	if err := startServer(); err != nil {
		log.Fatalf("start server failed: %v", err)
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if shouldRestart(event) {
					log.Printf("[dev] change detected: %s", event.Name)
					scheduleRestart()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("[dev] watcher error: %v", err)
			case <-ctx.Done():
				return
			}
		}
	}()

	<-ctx.Done()
	log.Println("[dev] shutting down...")

	mu.Lock()
	defer mu.Unlock()
	if cmd != nil {
		_ = terminateProcess(cmd)
	}
}

func watchGoDirs(watcher *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		if name == ".git" || name == ".gocache" || name == ".gomodcache" || name == "node_modules" {
			return filepath.SkipDir
		}
		return watcher.Add(path)
	})
}

func shouldRestart(event fsnotify.Event) bool {
	if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) == 0 {
		return false
	}
	return strings.HasSuffix(strings.ToLower(event.Name), ".go")
}

func terminateProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if err := cmd.Process.Kill(); err != nil {
		return err
	}
	return cmd.Wait()
}

func newDebouncer(delay time.Duration, fn func()) func() {
	var (
		mu    sync.Mutex
		timer *time.Timer
	)
	return func() {
		mu.Lock()
		defer mu.Unlock()
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(delay, fn)
	}
}

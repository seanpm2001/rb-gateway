package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/reviewboard/rb-gateway/api"
	"github.com/reviewboard/rb-gateway/config"
)

func main() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal("Could not watch configuration: ", err)
	}

	if err = watcher.Add(config.DefaultConfigPath); err != nil {
		log.Fatal("Could not watch configuration: ", err)
	}

	cfg, err := config.Load(config.DefaultConfigPath)
	if err != nil {
		log.Fatal("Could not load configuration: ", err)
	}

	api := api.New(*cfg)

	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	for {
		shouldExit := false
		log.Println("Starting rb-gateway server on port", cfg.Port)
		log.Println("Quit the server with CONTROL-C.")

		server := api.Serve()

		select {
		case <-watcher.Events:
			log.Println("Detected configuration change, reloading...")

		case watchErr := <-watcher.Errors:
			log.Fatal("Unexpected error: ", watchErr)

		case <-hup:
			log.Println("Received SIGHUP, reloading configuration...")

		case <-interrupt:
			shouldExit = true
			signal.Reset(os.Interrupt)
			log.Println("Received SIGINT, shutting down...")
			log.Println("CONTROL-C again to force quit.")
		}

		/*
		 * This allows us to give the server a grace period for finishing
		 * in-progress requests before it closes all connections.
		 */
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		server.Shutdown(ctx)
		cancel()
		log.Println("Server shut down.")

		if shouldExit {
			break
		}

		cfg, err = config.Load(config.DefaultConfigPath)
		if err != nil {
			log.Fatal("Could not load configuration: ", err)
		}
		api.SetConfig(*cfg)
	}
}

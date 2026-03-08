package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/liza-mas/liza/internal/embedded"
	"github.com/liza-mas/liza/internal/mcp"
)

func main() {
	var projectRoot string
	flag.StringVar(&projectRoot, "project-root", ".", "Path to Liza project root directory")
	flag.Parse()

	absProjectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving project root: %v\n", err)
		os.Exit(1)
	}

	logPath := filepath.Join(absProjectRoot, ".liza", "log.yaml")

	mcp.Version = embedded.Version
	mcp.BuildCommit = embedded.GitCommit

	server := mcp.NewServer(absProjectRoot, logPath)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Run()
	}()

	select {
	case err := <-errChan:
		if err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
		// Normal exit (EOF from stdin)
		os.Exit(0)
	case sig := <-sigChan:
		fmt.Fprintf(os.Stderr, "Received signal %v, shutting down...\n", sig)
		os.Exit(0)
	}
}

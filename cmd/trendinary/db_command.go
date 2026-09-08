package main

import (
	"context"
	"fmt"
	"os"
	"time"
)

func init() {
	args := os.Args[1:]
	if len(args) == 0 || args[0] != "db" {
		return
	}
	if len(args) != 2 || args[1] != "reset" {
		fmt.Fprintln(os.Stderr, "usage: trendinary db reset")
		os.Exit(2)
	}

	store, backend, err := openHistory()
	if err != nil {
		fmt.Fprintf(os.Stderr, "open %s database: %v\n", backend, err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	err = store.Reset(ctx)
	cancel()
	closeErr := store.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "reset %s database: %v\n", backend, err)
		os.Exit(1)
	}
	// The reset has already committed at this point. Some libSQL servers close a
	// WebSocket with EOF rather than a close frame; report it without turning a
	// successful destructive operation into a false failure.
	if closeErr != nil {
		fmt.Fprintf(os.Stderr, "warning: close %s database after committed reset: %v\n", backend, closeErr)
	}
	fmt.Fprintf(os.Stdout, "Trendinary %s database reset complete. Run Atlas before starting the application.\n", backend)
	os.Exit(0)
}

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
	if closeErr != nil {
		fmt.Fprintf(os.Stderr, "close %s database after reset: %v\n", backend, closeErr)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stdout, "Trendinary %s database reset complete.\n", backend)
	os.Exit(0)
}

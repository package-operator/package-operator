package main

import (
	"context"
	"os"
	"os/signal"
)

func main() {
	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt)
	os.Exit(Run(ctx, os.Stdin, os.Stdout, os.Stderr, os.Args[1:]))
}

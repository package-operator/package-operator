package main

import (
	"context"
	"os"
	"os/signal"

	"package-operator.run/package-operator/cmd/kubectl-package/command"
)

func main() {
	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt)
	os.Exit(command.Run(ctx, os.Stdin, os.Stdout, os.Stderr, os.Args[1:]))
}

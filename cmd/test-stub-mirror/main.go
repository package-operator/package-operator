package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// Test Stub to use as workload stand-in in integration tests.
func main() {
	if _, err := fmt.Fprintln(os.Stdout, "waiting for something interesting to happen..."); err != nil {
		panic(err)
	}

	// block forever
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	<-sigs
	if _, err := fmt.Fprintln(os.Stdout, "shutdown..."); err != nil {
		panic(err)
	}
}

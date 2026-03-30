package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/freQuensy23-coder/tgs/internal/cli"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := cli.Run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "tgs: %v\n", err)
		os.Exit(1)
	}
}

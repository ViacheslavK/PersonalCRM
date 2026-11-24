package main

import (
	"context"
	"fmt"
	"os"

	"personalcrm/internal/cli"
	"personalcrm/internal/tui"
)

func main() {
	if err := cli.Execute(context.Background(), os.Args[1:], cli.Options{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		RunTUI: tui.Run,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

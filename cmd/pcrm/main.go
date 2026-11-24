package main

import (
	"context"
	"fmt"
	"os"

	"personalcrm/internal/cli"
)

func main() {
	if err := cli.Execute(context.Background(), os.Args[1:], cli.Options{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

package main

import (
	"fmt"
	"os"

	"personalcrm/internal/platform"
)

const Version = "0.1.0-dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("pcrm %s\n", Version)
		return
	}

	paths, err := platform.ResolvePaths("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error resolving paths: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("PersonalCRM (%s)\nData dir:   %s\nConfig:     %s\nDatabase:   %s\nPortable:   %v\n",
		Version, paths.DataDir, paths.ConfigPath, paths.DBPath, paths.Portable)
}

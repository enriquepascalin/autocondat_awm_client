package main

import (
	"fmt"
	"os"
)

func main() {
	// No arguments → interactive shell menu.
	if len(os.Args) == 1 {
		runInteractiveMenu()
		return
	}

	// Arguments present → cobra dispatch.
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

// Package main is the entry point for the erg-server binary.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := Run(); err != nil {
		fmt.Fprintf(os.Stderr, "erg-server: %v\n", err)
		os.Exit(1)
	}
}

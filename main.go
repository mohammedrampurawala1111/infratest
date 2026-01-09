package main

import (
	"os"

	"github.com/infratest/infratest/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}


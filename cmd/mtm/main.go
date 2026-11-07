package main

import (
	"os"

	"github.com/loharrahulofficial-crypto/mtm/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}

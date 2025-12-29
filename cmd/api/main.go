package main

import (
	"os"

	"github.com/cun0/insider-case/internal/app"
)

var (
	version   = "dev"
	buildTime = ""
)

func main() {
	if err := app.Run(version, buildTime); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

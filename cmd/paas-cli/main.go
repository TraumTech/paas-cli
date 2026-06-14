package main

import (
	"context"
	"fmt"
	"os"

	"github.com/TraumTech/paas-cli/internal/app"
)

func main() {
	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "✗ "+err.Error())
		os.Exit(1)
	}
}

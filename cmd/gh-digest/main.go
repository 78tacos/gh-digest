package main

import (
	"os"

	"github.com/78tacos/gh-digest/internal/app"
)

func main() {
	os.Exit((&app.App{}).Run(os.Args[1:], os.Stdout, os.Stderr))
}

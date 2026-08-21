package main

import (
	"context"
	"os"
)

func main() {
	app := newApp(os.Stdout, os.Stderr)
	os.Exit(app.Run(context.Background(), os.Args[1:]))
}

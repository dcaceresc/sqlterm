package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sqlterm/internal/config"
	"sqlterm/internal/diagnostic"
	"sqlterm/internal/ui"
)

var version = "0.2.1"

func main() {
	if code := run(os.Args[1:], os.Stderr); code != 0 {
		os.Exit(code)
	}
}

func run(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flags.SetOutput(stderr)
	path := flags.String("config", config.DefaultPath(), "configuration file")
	ver := flags.Bool("version", false, "print version")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *ver {
		fmt.Println("sqlterm " + version)
		return 0
	}
	c, err := config.Load(*path)
	if err == nil {
		err = ui.New(*path, c).Run()
	}
	if err != nil {
		_ = diagnostic.LogError(*path, "Startup", err)
		fmt.Fprintln(stderr, "sqlterm:", err)
		return 1
	}
	return 0
}

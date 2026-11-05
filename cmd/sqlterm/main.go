package main

import (
	"flag"
	"fmt"
	"os"
	"sqlterm/internal/config"
	"sqlterm/internal/ui"
)

var version = "0.1.7"

func main() {
	path := flag.String("config", config.DefaultPath(), "configuration file")
	ver := flag.Bool("version", false, "print version")
	flag.Parse()
	if *ver {
		fmt.Println("sqlterm " + version)
		return
	}
	c, e := config.Load(*path)
	if e == nil {
		e = ui.New(*path, c).Run()
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, "sqlterm:", e)
		os.Exit(1)
	}
}

// Command board reports on every live coding-agent session running under cmux — Claude
// Code and maki, in one fleet (DESIGN.md §17).
//
// This file is dispatch and nothing else: each subcommand is a few lines that read
// arguments and hand off to a package. Logic here is logic that cannot be tested.
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/zalts1/dashy/internal/config"
	"github.com/zalts1/dashy/internal/hooks"
	"github.com/zalts1/dashy/internal/version"
	"github.com/zalts1/dashy/internal/watch"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		show()
		return
	}
	// Asked what it is, board answers on stdout and exits 0. An unrecognised command
	// still fails on stderr — being asked a question and being given a wrong one are
	// different events, and only one of them is an error.
	if helpRequested(args) {
		fmt.Println(usage)
		return
	}
	var err error
	switch args[0] {
	case "label":
		err = label(strings.Join(args[1:], " "))
	case "notify":
		hooks.Notify()
	case "watch":
		var iv time.Duration
		if iv, err = interval(args[1:]); err == nil {
			watch.Run(iv)
		}
	case "jump":
		err = jump(strings.Join(args[1:], " "))
	case "todo":
		err = todo(args[1:])
	case "install-hooks":
		err = hooks.Install()
	case "uninstall-hooks":
		err = hooks.Uninstall()
	case "doctor":
		diagnose()
	case "version":
		fmt.Print(version.Format(version.Report()))
	default:
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "board:", err)
		os.Exit(1)
	}
}

func interval(args []string) (time.Duration, error) {
	if len(args) == 0 {
		return config.Load().Poll(), nil
	}
	d, err := time.ParseDuration(args[0])
	if err != nil || d < time.Second {
		return 0, fmt.Errorf("bad interval %q (try 10s, 30s, 1m)", args[0])
	}
	return d, nil
}

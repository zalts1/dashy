// Command board reports on every live Claude Code session running under cmux.
//
// This file is dispatch and nothing else: each subcommand is a few lines that read
// arguments and hand off to a package. Logic here is logic that cannot be tested.
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"board/internal/config"
	"board/internal/hooks"
	"board/internal/watch"
)

const usage = `usage: board | board watch [interval] | board jump <substring> | board label "<text>"
       board todo | board todo "<text>" | board todo done <text or id> | board install-hooks`

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		show()
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

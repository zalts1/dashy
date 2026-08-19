package main

const usage = `usage: board | board watch [interval] | board jump <substring> | board label "<text>"
       board todo | board todo "<text>" | board todo done <text or id>
       board editor | board editor <name> | board editor auto
       board install-hooks | board uninstall-hooks | board version | board doctor`

// helpRequested reports whether the arguments are a question rather than work.
//
// `-h` and `--help` count in any position, because `todo` joins its arguments into text:
// a flag that quietly became a todo item is a worse answer than a redundant usage line.
// The bare word counts only first, since a todo may legitimately be about help.
func helpRequested(args []string) bool {
	for i, a := range args {
		if a == "-h" || a == "--help" || (i == 0 && a == "help") {
			return true
		}
	}
	return false
}

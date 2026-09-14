// Command encre is the developer CLI for github.com/oioio-space/encre: it
// analyses words for their spelling traps and reports the tool's version.
package main

import (
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// Command encre is the CLI entry point for github.com/oioio-space/encre.
package main

import (
	"fmt"
	"os"

	encre "github.com/oioio-space/encre"
)

func main() {
	name := ""
	if len(os.Args) > 1 {
		name = os.Args[1]
	}
	fmt.Println(encre.Greet(name))
}

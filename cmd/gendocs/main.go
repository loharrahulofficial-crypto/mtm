// gendocs renders the man pages for the whole command tree with the
// Cobra docs generator. Usage: gendocs [outdir]   (default: man/)
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra/doc"

	"github.com/loharrahulofficial-crypto/mtm/internal/cli"
)

func main() {
	out := "man"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		panic(err)
	}
	root := cli.NewRootCmd()
	root.DisableAutoGenTag = true // deterministic output; tag shows the date instead
	header := &doc.GenManHeader{
		Title:   "MTM",
		Section: "1",
		Source:  "mtm " + cli.VersionString(),
		Manual:  "User Commands",
	}
	if err := doc.GenManTree(root, header, out); err != nil {
		panic(err)
	}
	fmt.Printf("man pages written to %s\n", out)
}

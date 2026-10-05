// Command isorion is the Universal Infrastructure Compiler CLI.
//
//	isorion fusion <file.isor> [-t target[,target...]] [-o output-dir]
//	isorion validate <file.isor>
//	isorion targets
//	isorion version
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/isorion-lang/isorion/pkg/emit"
)

// Overridable at link time: -X main.version=... (goreleaser).
var (
	version = "0.1.0"
	commit  = ""
	date    = ""
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "fusion":
		err = cmdFusion(os.Args[2:])
	case "validate":
		err = cmdValidate(os.Args[2:])
	case "diff":
		err = cmdDiff(os.Args[2:])
	case "fmt":
		err = cmdFmt(os.Args[2:])
	case "import":
		err = cmdImport(os.Args[2:])
	case "targets":
		err = cmdTargets(os.Args[2:])
	case "version", "--version", "-v":
		extra := ""
		if commit != "" {
			extra = fmt.Sprintf(" (%s, %s)", commit, date)
		}
		fmt.Printf("isorion %s%s — Universal Infrastructure Compiler\n", version, extra)
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "isorion: unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if errors.Is(err, errDiffer) {
		os.Exit(1) // diff found changes — summary already printed
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "isorion: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `ISORION — Universal Infrastructure Compiler
Write infrastructure once in .isor, deploy to many targets.

Usage:
  isorion fusion <file.isor> [-t target[,target...]] [-o output-dir]
      Compile a .isor file to one or more targets.
      Default output dir: ./output ; default targets: all.
  isorion validate <file.isor>
      Parse and expand a .isor file, report errors only.
  isorion diff <file.isor> [-t targets] [-o dir] [-v]
      Compare emitted output with an existing output dir (exit 1 on changes).
  isorion import <file> [-f format] [-o app.isor]
      Lift an existing infra file (docker-compose.yml...) back to .isor.
  isorion fmt <file.isor> [-w]
      Canonicalize a .isor file (stdout, or in-place with -w).
  isorion targets
      List available compilation targets.
  isorion version
      Print version.

Targets (community edition): %s
`, emitList())
}

func emitList() string {
	names := emit.Names()
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}

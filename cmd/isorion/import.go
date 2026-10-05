package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/isorion-lang/isorion/pkg/isorion"
)

// cmdImport lifts an existing infra file (docker-compose.yml, ...) back to
// a canonical .isor document via the isor target — the reverse-compiler
// pipeline: source format → model.Infra → YAML++.
func cmdImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	format := fs.String("f", "docker-compose", "input format ("+joinNames()+")")
	out := fs.String("o", "", "output .isor file (default: stdout)")

	flagArgs, posArgs := splitArgs(args)
	if err := fs.Parse(append(flagArgs, posArgs...)); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("import requires an infra file\n\nusage: isorion import <file> [-f format] [-o app.isor]")
	}
	srcPath := fs.Arg(0)

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	isorDoc, _, err := isorion.Import(*format, data, filepath.Base(srcPath))
	if err != nil {
		return err
	}

	if *out == "" {
		_, err = os.Stdout.Write(isorDoc)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(*out, isorDoc, 0o644); err != nil {
		return err
	}
	fmt.Printf("import: %s → %s\n", srcPath, *out)
	return nil
}

func joinNames() string {
	return strings.Join(isorion.Importers(), ", ")
}

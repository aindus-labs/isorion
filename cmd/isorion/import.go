package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/isorion-lang/isorion/pkg/emit"
	"github.com/isorion-lang/isorion/pkg/lang"
	"github.com/isorion-lang/isorion/pkg/parse"
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

	imp, err := parse.Get(*format)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	infra, err := imp.Parse(data, filepath.Base(srcPath))
	if err != nil {
		return err
	}

	files, warnings, err := emit.Run("isor", &lang.Document{}, infra)
	if err != nil {
		return err
	}
	printWarnings(warnings)

	if *out == "" {
		for _, f := range files {
			if _, err := os.Stdout.Write(f.Content); err != nil {
				return err
			}
		}
		return nil
	}
	for _, f := range files {
		dest := *out
		if len(files) > 1 {
			dest = filepath.Join(*out, f.Path)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, f.Content, 0o644); err != nil {
			return err
		}
		fmt.Printf("import: %s → %s\n", srcPath, dest)
	}
	return nil
}

func joinNames() string {
	names := parse.Names()
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}

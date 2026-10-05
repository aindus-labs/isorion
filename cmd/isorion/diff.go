package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/isorion-lang/isorion/pkg/isorion"
)

// errDiffer exits the process with code 1 after the diff summary —
// the standard convention for diff tools (git diff --exit-code).
var errDiffer = errors.New("outputs differ")

// cmdDiff compares what fusion would emit with the existing output
// directory — auditable review without touching artifacts.
func cmdDiff(args []string) error {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	targetsFlag := fs.String("t", "", "comma-separated targets (default: all)")
	outDir := fs.String("o", "output", "output directory to compare against")
	verbose := fs.Bool("v", false, "print a unified diff per changed file")
	var sets strList
	fs.Var(&sets, "set", "override a variable: --set name=value (repeatable)")
	profile := fs.String("profile", "", "merge a profiles: entry into variables:")

	flagArgs, posArgs := splitArgs(args)
	if err := fs.Parse(append(flagArgs, posArgs...)); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("diff requires a .isor file\n\nusage: isorion diff <file.isor> [-t targets] [-o dir] [-v] [--set k=v] [--profile name]")
	}
	srcPath := fs.Arg(0)

	setMap, err := parseSets(sets)
	if err != nil {
		return err
	}
	src, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	doc, infra, cwarns, err := isorion.Validate(srcPath, src,
		isorion.Options{Profile: *profile, Set: setMap})
	printWarnings(cwarns)
	if err != nil {
		return err
	}

	targets := isorion.Targets()
	if *targetsFlag != "" {
		targets = strings.Split(*targetsFlag, ",")
	}

	changed := 0
	for _, t := range targets {
		t = strings.TrimSpace(t)
		files, warnings, err := isorion.Emit(t, doc, infra)
		if err != nil {
			return fmt.Errorf("target %s: %w", t, err)
		}
		printWarnings(warnings)
		for _, f := range files {
			dest := filepath.Join(*outDir, t, f.Path)
			old, rerr := os.ReadFile(dest)
			switch {
			case errors.Is(rerr, os.ErrNotExist):
				changed++
				fmt.Printf("+ %s (new, %d lines)\n", dest, countLines(f.Content))
			case rerr != nil:
				return rerr
			case !bytes.Equal(old, f.Content):
				changed++
				add, del := diffStats(old, f.Content)
				fmt.Printf("~ %s (+%d/-%d lines)\n", dest, add, del)
				if *verbose {
					fmt.Print(unifiedDiff(dest, old, f.Content))
				}
			}
		}
	}
	if changed > 0 {
		fmt.Printf("diff: %d file(s) differ from %s\n", changed, *outDir)
		return errDiffer
	}
	fmt.Printf("diff: no differences vs %s\n", *outDir)
	return nil
}

func countLines(b []byte) int {
	if len(b) == 0 {
		return 0
	}
	return bytes.Count(b, []byte("\n")) + 1
}

// diffStats reports added/removed line counts between old and new content.
func diffStats(old, new []byte) (add, del int) {
	o := strings.Split(strings.TrimRight(string(old), "\n"), "\n")
	n := strings.Split(strings.TrimRight(string(new), "\n"), "\n")
	lcs := lcsTable(o, n)
	i, j := len(o), len(n)
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && o[i-1] == n[j-1]:
			i--
			j--
		case j > 0 && (i == 0 || lcs[i][j-1] >= lcs[i-1][j]):
			add++
			j--
		default:
			del++
			i--
		}
	}
	return add, del
}

// lcsTable is the classic longest-common-subsequence DP table.
func lcsTable(a, b []string) [][]int {
	t := make([][]int, len(a)+1)
	for i := range t {
		t[i] = make([]int, len(b)+1)
	}
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				t[i][j] = t[i-1][j-1] + 1
			} else if t[i-1][j] >= t[i][j-1] {
				t[i][j] = t[i-1][j]
			} else {
				t[i][j] = t[i][j-1]
			}
		}
	}
	return t
}

// unifiedDiff renders a minimal unified diff (context-free -/+ lines).
func unifiedDiff(path string, old, new []byte) string {
	o := strings.Split(strings.TrimRight(string(old), "\n"), "\n")
	n := strings.Split(strings.TrimRight(string(new), "\n"), "\n")
	lcs := lcsTable(o, n)
	var b strings.Builder
	fmt.Fprintf(&b, "--- %s (on disk)\n+++ %s (emitted)\n", path, path)
	i, j := len(o), len(n)
	var ops []string
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && o[i-1] == n[j-1]:
			i--
			j--
		case j > 0 && (i == 0 || lcs[i][j-1] >= lcs[i-1][j]):
			ops = append(ops, "+"+n[j-1])
			j--
		default:
			ops = append(ops, "-"+o[i-1])
			i--
		}
	}
	for k := len(ops) - 1; k >= 0; k-- {
		b.WriteString(ops[k])
		b.WriteByte('\n')
	}
	return b.String()
}

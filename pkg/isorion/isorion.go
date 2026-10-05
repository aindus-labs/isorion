// Package isorion is the stable public API of the compiler — the single
// entry point for embedding ISORION in other Go programs (level 1 of the
// interface contract). It composes pkg/lang → pkg/model → pkg/emit /
// pkg/parse behind three calls: Compile, Import, Validate.
//
//	doc, infra, err := isorion.Validate("app.isor", src, isorion.Options{})
//	res, err := isorion.Compile("app.isor", src, []string{"kubernetes"}, opts)
//	isor, _, err := isorion.Import("docker-compose", composeYAML, "docker-compose.yml")
package isorion

import (
	"fmt"
	"strings"

	"github.com/isorion-lang/isorion/pkg/emit"
	"github.com/isorion-lang/isorion/pkg/lang"
	"github.com/isorion-lang/isorion/pkg/model"
	"github.com/isorion-lang/isorion/pkg/parse"
)

// Options mirrors CLI injection: --profile / --set name=value.
type Options struct {
	Profile string            // `profiles:` entry merged over `variables:`
	Set     map[string]string // literal overrides applied last
}

// Result is the output of Compile: one File per emitted artifact plus
// capability warnings aggregated across targets.
type Result struct {
	Files    []emit.File
	Warnings []string
}

// Compile parses + expands a .isor source and emits it to each target
// (composite `base+tx` allowed). Fail-fast: the first error aborts.
func Compile(filename string, src []byte, targets []string, opts Options) (*Result, error) {
	doc, infra, err := build(filename, src, opts)
	if err != nil {
		return nil, err
	}
	res := &Result{}
	for _, t := range targets {
		files, warns, err := emit.Run(t, doc, infra)
		if err != nil {
			return nil, fmt.Errorf("target %s: %w", t, err)
		}
		res.Files = append(res.Files, files...)
		res.Warnings = append(res.Warnings, warns...)
	}
	return res, nil
}

// Validate parses + expands + model-checks a .isor source without
// emitting. Returns the document and the normalized infra on success.
func Validate(filename string, src []byte, opts Options) (*lang.Document, *model.Infra, error) {
	return build(filename, src, opts)
}

// Import lifts an existing infra artifact back to the model and returns
// its canonical .isor serialization (reverse-compiler pipeline).
func Import(format string, data []byte, filename string) (isorDoc []byte, infra *model.Infra, err error) {
	imp, err := parse.Get(format)
	if err != nil {
		return nil, nil, err
	}
	infra, err = imp.Parse(data, filename)
	if err != nil {
		return nil, nil, err
	}
	files, _, err := emit.Run("isor", &lang.Document{}, infra)
	if err != nil {
		return nil, nil, err
	}
	if len(files) != 1 {
		return nil, nil, fmt.Errorf("isor emit produced %d files", len(files))
	}
	return files[0].Content, infra, nil
}

// Targets lists registered emitter names; Importers lists import formats.
func Targets() []string   { return emit.Names() }
func Importers() []string { return parse.Names() }

// build is the shared front-end: source → Document → Infra, with model
// errors position-prefixed when the offending section is locatable.
func build(filename string, src []byte, opts Options) (*lang.Document, *model.Infra, error) {
	doc, err := lang.LoadWith(filename, src, lang.Options(opts))
	if err != nil {
		return nil, nil, err
	}
	infra, err := model.FromBody(doc.Version, doc.Body)
	if err != nil {
		return nil, nil, posify(doc, err)
	}
	cwarns, err := model.CheckConstraints(doc.Body, doc.Constraints)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", doc.Source, err)
	}
	_ = cwarns // advisory/mandatory constraint warnings surface via Result in callers that opt in
	return doc, infra, nil
}

// posify prefixes a model-build error with the source position of the
// offending top-level section when known: `file:line:col: model: …`.
func posify(doc *lang.Document, err error) error {
	section := err.Error()
	if i := strings.IndexAny(section, ".:"); i >= 0 {
		section = section[:i]
	}
	if pos, ok := doc.Positions[section]; ok && pos.Line > 0 {
		return fmt.Errorf("%s:%d:%d: %s", doc.Source, pos.Line, pos.Col, err)
	}
	return fmt.Errorf("%s: %w", doc.Source, err)
}

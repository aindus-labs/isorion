package emit

import (
	"reflect"
	"strings"
	"testing"

	"github.com/isorion-lang/isorion/pkg/lang"
	"github.com/isorion-lang/isorion/pkg/model"
)

// TestIsorRoundTrip: the `isor` target must emit a document that re-parses
// into an identical model — parse(emit(x)) == x on model.Infra (I4
// invariant 3).
func TestIsorRoundTrip(t *testing.T) {
	doc, infra := loadFixture(t)

	files, warnings, err := Run("isor", doc, infra)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("isor target must not warn: %v", warnings)
	}
	if len(files) != 1 || !strings.HasSuffix(files[0].Path, ".isor") {
		t.Fatalf("expected one .isor file, got %+v", files)
	}

	doc2, err := lang.LoadWith("roundtrip.isor", files[0].Content, lang.Options{})
	if err != nil {
		t.Fatalf("emitted .isor does not re-parse: %v\n%s", err, files[0].Content)
	}
	infra2, err := model.FromBody(doc2.Version, doc2.Body)
	if err != nil {
		t.Fatal(err)
	}

	if infra2.ProjectName != infra.ProjectName || infra2.Version != infra.Version {
		t.Fatalf("project/version changed: %+v vs %+v", infra2, infra)
	}
	if !reflect.DeepEqual(infra2.ServiceNames, infra.ServiceNames) {
		t.Fatalf("service names changed: %v vs %v", infra2.ServiceNames, infra.ServiceNames)
	}
	for _, name := range infra.ServiceNames {
		if !reflect.DeepEqual(infra2.Services[name], infra.Services[name]) {
			t.Fatalf("service %s changed:\n%+v\n%+v", name, infra2.Services[name], infra.Services[name])
		}
	}
	if !reflect.DeepEqual(infra2.Networks, infra.Networks) {
		t.Fatalf("networks changed: %v vs %v", infra2.Networks, infra.Networks)
	}
}

// TestIsorExtraPassthrough: unknown service/top-level fields survive a
// .isor → model → .isor round-trip.
func TestIsorExtraPassthrough(t *testing.T) {
	src := `version: "1.0"
project: { name: x }
services:
  a:
    image: nginx
    healthcheck: { test: "curl -f localhost" }
extra_top: { keep: me }
`
	doc, err := lang.LoadWith("x.isor", []byte(src), lang.Options{})
	if err != nil {
		t.Fatal(err)
	}
	infra, err := model.FromBody(doc.Version, doc.Body)
	if err != nil {
		t.Fatal(err)
	}
	files, _, err := Run("isor", doc, infra)
	if err != nil {
		t.Fatal(err)
	}
	out := string(files[0].Content)
	if !strings.Contains(out, "healthcheck") || !strings.Contains(out, "extra_top") {
		t.Fatalf("Extra fields lost:\n%s", out)
	}
}

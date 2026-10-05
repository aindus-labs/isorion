package model

import (
	"strings"
	"testing"

	"github.com/isorion-lang/isorion/pkg/lang"
)

func bodyFixture() map[string]any {
	return map[string]any{
		"services": map[string]any{
			"api": map[string]any{"replicas": int64(3), "image": "nginx"},
			"db":  map[string]any{"replicas": int64(0), "image": "postgres"},
		},
	}
}

func TestConstraintsPass(t *testing.T) {
	cs := []lang.Constraint{
		{Expr: "services.*.image != \"\"", Level: "mandatory"},
		{Expr: "services.api.replicas >= 1", Level: "mandatory"},
	}
	w, err := CheckConstraints(bodyFixture(), cs)
	if err != nil || len(w) != 0 {
		t.Fatalf("warnings=%v err=%v", w, err)
	}
}

func TestConstraintsMandatoryFail(t *testing.T) {
	cs := []lang.Constraint{
		{Expr: "services.*.replicas >= 1", Level: "mandatory"},
	}
	_, err := CheckConstraints(bodyFixture(), cs)
	if err == nil || !strings.Contains(err.Error(), "db") {
		t.Fatalf("expected mandatory failure naming db, err = %v", err)
	}
}

func TestConstraintsAdvisory(t *testing.T) {
	cs := []lang.Constraint{
		{Expr: "services.*.replicas >= 4", Level: "advisory"},
	}
	w, err := CheckConstraints(bodyFixture(), cs)
	if err != nil {
		t.Fatalf("advisory must not fail: %v", err)
	}
	if len(w) != 2 {
		t.Fatalf("expected 2 advisory warnings (api=3,db=0 <4), got %v", w)
	}
}

func TestConstraintsMissingPath(t *testing.T) {
	cs := []lang.Constraint{
		{Expr: "services.*.healthcheck != \"\"", Level: "mandatory"},
	}
	if _, err := CheckConstraints(bodyFixture(), cs); err == nil {
		t.Fatal("expected missing-path failure")
	}
	cs[0].Level = "advisory"
	if w, err := CheckConstraints(bodyFixture(), cs); err != nil || len(w) == 0 {
		t.Fatalf("advisory missing path should warn, w=%v err=%v", w, err)
	}
}

func TestConstraintsExprForm(t *testing.T) {
	cs := []lang.Constraint{
		{Expr: "len(\"abc\") == 3", Level: "mandatory"},
	}
	if _, err := CheckConstraints(bodyFixture(), cs); err != nil {
		t.Fatalf("expr form: %v", err)
	}
}

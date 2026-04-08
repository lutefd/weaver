package git

import (
	"reflect"
	"testing"
)

func TestParseBranches(t *testing.T) {
	t.Parallel()

	output := "* main\n  feature-a\n  feature-b\n"
	want := []string{"main", "feature-a", "feature-b"}
	if got := ParseBranches(output); !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseBranches() = %#v, want %#v", got, want)
	}
}

func TestParseCurrentBranch(t *testing.T) {
	t.Parallel()

	if got := ParseCurrentBranch("feature-a\n"); got != "feature-a" {
		t.Fatalf("ParseCurrentBranch() = %q, want feature-a", got)
	}
}

package git

import (
	"reflect"
	"strings"
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

func TestParseAheadBehind(t *testing.T) {
	t.Parallel()

	ahead, behind, err := ParseAheadBehind("3\t7\n")
	if err != nil {
		t.Fatalf("ParseAheadBehind() error = %v", err)
	}
	if ahead != 3 || behind != 7 {
		t.Fatalf("ParseAheadBehind() = %d, %d, want 3, 7", ahead, behind)
	}
}

func TestParseAheadBehindErrors(t *testing.T) {
	t.Parallel()

	if _, _, err := ParseAheadBehind("3"); err == nil || !strings.Contains(err.Error(), "unexpected ahead/behind output") {
		t.Fatalf("ParseAheadBehind() error = %v, want unexpected output error", err)
	}
	if _, _, err := ParseAheadBehind("x 7"); err == nil || !strings.Contains(err.Error(), `parse ahead count "x"`) {
		t.Fatalf("ParseAheadBehind() error = %v, want ahead parse error", err)
	}
	if _, _, err := ParseAheadBehind("3 y"); err == nil || !strings.Contains(err.Error(), `parse behind count "y"`) {
		t.Fatalf("ParseAheadBehind() error = %v, want behind parse error", err)
	}
}

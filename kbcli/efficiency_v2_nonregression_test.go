package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kanban/core/models"
)

// TestEfficiencyV2DoesNotReadLegacyCommitAncientMinutes asserts the v2
// algorithmic estimator never sources baseline minutes from the legacy
// `commit_ancient_minutes` field on the `commits` table.
//
// Implementation note: we scan every efficiency_v2_*.go file (excluding
// fixture seed data and intentional doc comments) for any direct access
// to `commit.CommitAncientMinutes`. Comments and the fixture seeding code
// are skipped explicitly.
func TestEfficiencyV2DoesNotReadLegacyCommitAncientMinutes(t *testing.T) {
	matches, err := filepath.Glob("efficiency_v2_*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("expected efficiency_v2 source files")
	}

	fset := token.NewFileSet()
	for _, path := range matches {
		if strings.Contains(path, "_test.go") {
			continue
		}
		if strings.HasSuffix(path, "_fixture.go") {
			// seeding fixture data into commits is unavoidable
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		// Strip block and line comments before searching.
		_, err = parser.ParseFile(fset, path, data, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		// Remove comment lines and string literals roughly.
		stripped := stripCommentsAndStrings(string(data))
		if strings.Contains(stripped, "CommitAncientMinutes") {
			t.Errorf("%s: v2 code must not reference legacy CommitAncientMinutes outside fixture seeding", path)
		}
	}
}

func stripCommentsAndStrings(src string) string {
	var sb strings.Builder
	i := 0
	for i < len(src) {
		c := src[i]
		// line comment
		if c == '/' && i+1 < len(src) && src[i+1] == '/' {
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		}
		// block comment
		if c == '/' && i+1 < len(src) && src[i+1] == '*' {
			i += 2
			for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
				i++
			}
			i += 2
			continue
		}
		// string literal
		if c == '"' {
			i++
			for i < len(src) && src[i] != '"' {
				if src[i] == '\\' && i+1 < len(src) {
					i++
				}
				i++
			}
			i++
			continue
		}
		// raw string
		if c == '`' {
			i++
			for i < len(src) && src[i] != '`' {
				i++
			}
			i++
			continue
		}
		sb.WriteByte(c)
		i++
	}
	return sb.String()
}

// TestEfficiencyV2BaselineAExecDoesNotReadCommitAncient asserts the public
// signature of `computeEfficiencyV2BaselineExec` operates on commit
// DiffLines only.
func TestEfficiencyV2BaselineAExecDoesNotReadCommitAncient(t *testing.T) {
	need := models.Need{NeedId: "n-1", TouchedFiles: efficiencyV2StringJSON([]string{"src/app.go"})}
	// CommitAncientMinutes set to a wildly large value; v2 exec must ignore it.
	commit := models.Commit{CommitId: "c1", DiffLines: 100, CommitAncientMinutes: 99999}
	coefs := DefaultEfficiencyV2BaselineACoefficients()
	exec, _ := computeEfficiencyV2BaselineExec(need, []models.Commit{commit}, coefs)
	if exec == nil {
		t.Fatalf("exec should be non-nil")
	}
	// expected = 100 / (100/480) + 1*30 = 480 + 30 = 510, definitely << 99999
	if *exec >= 99999 {
		t.Fatalf("exec = %.2f looks like it absorbed CommitAncientMinutes", *exec)
	}
}

// TestLegacyEfficiencyCmdStillRegistered confirms the legacy efficiency
// command is preserved alongside the new v2 path.
func TestLegacyEfficiencyCmdStillRegistered(t *testing.T) {
	if !validTaskTypes["efficiency"] {
		t.Fatalf("legacy `efficiency` task type must remain registered")
	}
	if !validTaskTypes["efficiency-v2"] {
		t.Fatalf("`efficiency-v2` task type should be registered")
	}
}

package features_test

import (
	"testing"

	. "github.com/jinkp/atlassian-go-mcp/internal/mcp/features"
)

// TestParse covers F1.1–F1.12
func TestParse(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantModules   map[string]AccessLevel
		wantAllModulesRW bool
	}{
		{
			name:             "F1.1 empty string → all modules RW",
			input:            "",
			wantAllModulesRW: true,
		},
		{
			name:             "F1.2 'all' → all modules RW",
			input:            "all",
			wantAllModulesRW: true,
		},
		{
			name:  "F1.3 'jira' → only jira=RW",
			input: "jira",
			wantModules: map[string]AccessLevel{
				"jira": AccessReadWrite,
			},
		},
		{
			name:  "F1.4 'jira-read' → jira=Read",
			input: "jira-read",
			wantModules: map[string]AccessLevel{
				"jira": AccessRead,
			},
		},
		{
			name:  "F1.5 'jira-write' → jira=Write",
			input: "jira-write",
			wantModules: map[string]AccessLevel{
				"jira": AccessWrite,
			},
		},
		{
			name:  "F1.6 'jira,agile-read' → jira=RW agile=Read",
			input: "jira,agile-read",
			wantModules: map[string]AccessLevel{
				"jira":  AccessReadWrite,
				"agile": AccessRead,
			},
		},
		{
			name:  "F1.7 'goals,metrics' → both RW",
			input: "goals,metrics",
			wantModules: map[string]AccessLevel{
				"goals":   AccessReadWrite,
				"metrics": AccessReadWrite,
			},
		},
		{
			name:        "F1.8 'unknown' → empty set",
			input:       "unknown",
			wantModules: map[string]AccessLevel{},
		},
		{
			name:  "F1.9 'jira,unknown,agile' → jira+agile RW, unknown ignored",
			input: "jira,unknown,agile",
			wantModules: map[string]AccessLevel{
				"jira":  AccessReadWrite,
				"agile": AccessReadWrite,
			},
		},
		{
			name:        "F1.10 'JIRA' is case-sensitive → empty set",
			input:       "JIRA",
			wantModules: map[string]AccessLevel{},
		},
		{
			name:  "F1.11 'jira-read,jira-write' → accumulated to RW",
			input: "jira-read,jira-write",
			wantModules: map[string]AccessLevel{
				"jira": AccessReadWrite,
			},
		},
		{
			name:  "F1.12 whitespace trimmed",
			input: "  jira , agile ",
			wantModules: map[string]AccessLevel{
				"jira":  AccessReadWrite,
				"agile": AccessReadWrite,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fs := Parse(tc.input)
			if fs == nil {
				t.Fatal("Parse returned nil")
			}
			if tc.wantAllModulesRW {
				allMods := []string{"jira", "agile", "goals", "metrics", "releases", "projects", "teams"}
				for _, mod := range allMods {
					if !fs.IsEnabled(mod, false) {
						t.Errorf("module %q: read not enabled, want enabled", mod)
					}
					if !fs.IsEnabled(mod, true) {
						t.Errorf("module %q: write not enabled, want enabled", mod)
					}
				}
				return
			}
			// check exact module set
			allMods := []string{"jira", "agile", "goals", "metrics", "releases", "projects", "teams"}
			for _, mod := range allMods {
				wantLevel, wantPresent := tc.wantModules[mod]
				gotRead := fs.IsEnabled(mod, false)
				gotWrite := fs.IsEnabled(mod, true)
				if !wantPresent {
					if gotRead || gotWrite {
						t.Errorf("module %q: expected disabled, got read=%v write=%v", mod, gotRead, gotWrite)
					}
					continue
				}
				wantRead := wantLevel&AccessRead != 0
				wantWrite := wantLevel&AccessWrite != 0
				if gotRead != wantRead {
					t.Errorf("module %q: read: got %v, want %v", mod, gotRead, wantRead)
				}
				if gotWrite != wantWrite {
					t.Errorf("module %q: write: got %v, want %v", mod, gotWrite, wantWrite)
				}
			}
		})
	}
}

// TestIsEnabled covers F2.1–F2.10
func TestIsEnabled(t *testing.T) {
	tests := []struct {
		name      string
		parseStr  string
		module    string
		write     bool
		want      bool
		nilFS     bool
	}{
		{"F2.1 jira RW → read=true", "jira", "jira", false, true, false},
		{"F2.2 jira RW → write=true", "jira", "jira", true, true, false},
		{"F2.3 jira-read → read=true", "jira-read", "jira", false, true, false},
		{"F2.4 jira-read → write=false", "jira-read", "jira", true, false, false},
		{"F2.5 jira-write → read=false", "jira-write", "jira", false, false, false},
		{"F2.6 jira-write → write=true", "jira-write", "jira", true, true, false},
		{"F2.7 agile enabled → jira read=false", "agile", "jira", false, false, false},
		{"F2.8 all → teams read=true", "", "teams", false, true, false},
		{"F2.9 unknown → jira read=false", "unknown", "jira", false, false, false},
		{"F2.10 nil fs → jira read=true", "", "jira", false, true, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var fs *FeatureSet
			if !tc.nilFS {
				fs = Parse(tc.parseStr)
			}
			got := fs.IsEnabled(tc.module, tc.write)
			if got != tc.want {
				t.Errorf("IsEnabled(%q, %v): got %v, want %v", tc.module, tc.write, got, tc.want)
			}
		})
	}
}

// TestEnabledToolCount covers F3.1–F3.14
func TestEnabledToolCount(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"F3.1 all → 37", "all", 37},
		{"F3.2 empty → 37", "", 37},
		{"F3.3 jira → 7", "jira", 7},
		{"F3.4 jira-read → 4", "jira-read", 4},
		{"F3.5 jira-write → 3", "jira-write", 3},
		{"F3.6 agile → 8", "agile", 8},
		{"F3.7 goals → 6", "goals", 6},
		{"F3.8 metrics → 4", "metrics", 4},
		{"F3.9 goals,metrics → 10", "goals,metrics", 10},
		{"F3.10 releases → 5", "releases", 5},
		{"F3.11 projects → 4", "projects", 4},
		{"F3.12 teams → 3", "teams", 3},
		{"F3.13 unknown → 0", "unknown", 0},
		{"F3.14 jira,agile → 15", "jira,agile", 15},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fs := Parse(tc.input)
			got := fs.EnabledToolCount()
			if got != tc.want {
				t.Errorf("Parse(%q).EnabledToolCount(): got %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

// TestString covers F4.1–F4.5
func TestString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"F4.1 all → 'all'", "all", "all"},
		{"F4.2 jira → 'jira'", "jira", "jira"},
		{"F4.3 jira-read → 'jira-read'", "jira-read", "jira-read"},
		{"F4.4 jira,agile-read → 'jira,agile-read'", "jira,agile-read", "jira,agile-read"},
		{"F4.5 empty → 'all'", "", "all"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Parse(tc.input).String()
			if got != tc.want {
				t.Errorf("Parse(%q).String(): got %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestParse_Triangulate: accumulated access levels
func TestParse_Triangulate(t *testing.T) {
	fs := Parse("jira-read,jira-write")
	if got := fs.EnabledToolCount(); got != 7 {
		t.Errorf("jira-read,jira-write EnabledToolCount: got %d, want 7", got)
	}
	if !fs.IsEnabled("jira", false) {
		t.Error("jira-read,jira-write: read should be enabled")
	}
	if !fs.IsEnabled("jira", true) {
		t.Error("jira-read,jira-write: write should be enabled")
	}
}

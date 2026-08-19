package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jinkp/atlassian-go-mcp/internal/tui"
)

// helper: send a key rune to the model
func pressRune(m tui.Model, r rune) tui.Model {
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	return m2.(tui.Model)
}

// helper: send a named key
func pressKey(m tui.Model, k tea.KeyType) tui.Model {
	m2, _ := m.Update(tea.KeyMsg{Type: k})
	return m2.(tui.Model)
}

// --- T1: Initial state ---

func TestNewModel_InitialState(t *testing.T) {
	m := tui.NewModel()

	// T1.1 screen = ScreenModules
	if m.Screen() != tui.ScreenModules {
		t.Errorf("Screen: got %d, want ScreenModules(%d)", m.Screen(), tui.ScreenModules)
	}

	// T1.3 cursor = 0
	if m.Cursor() != 0 {
		t.Errorf("Cursor: got %d, want 0", m.Cursor())
	}

	// T1.4 preview = lean list (confluence disabled by default, so not "all")
	wantPreview := "jira,agile,goals,metrics,releases,projects,teams,bitbucket"
	if m.Preview() != wantPreview {
		t.Errorf("Preview: got %q, want %q", m.Preview(), wantPreview)
	}

	// T1.2 + T1.5: 9 modules (8 enabled RW + confluence disabled)
	wantModules := []struct {
		name    string
		enabled bool
	}{
		{"jira", true},
		{"agile", true},
		{"goals", true},
		{"metrics", true},
		{"releases", true},
		{"projects", true},
		{"teams", true},
		{"bitbucket", true},
		{"confluence", false}, // lean default: off
	}
	mods := m.Modules()
	if len(mods) != len(wantModules) {
		t.Fatalf("Modules len: got %d, want %d", len(mods), len(wantModules))
	}
	for i, want := range wantModules {
		mod := mods[i]
		if mod.Name != want.name {
			t.Errorf("modules[%d].Name: got %q, want %q", i, mod.Name, want.name)
		}
		if mod.Enabled != want.enabled {
			t.Errorf("modules[%d].Enabled: got %v, want %v", i, mod.Enabled, want.enabled)
		}
		if want.enabled && mod.Access != tui.AccessReadWrite {
			t.Errorf("modules[%d].Access: got %d, want AccessReadWrite", i, mod.Access)
		}
	}
}

// --- T1b: Logo on the modules screen ---

func TestModulesViewShowsLogo(t *testing.T) {
	m := tui.NewModel() // starts on ScreenModules
	out := m.View()
	if !strings.Contains(out, "█") {
		t.Error("expected the ASCII logo block in the modules view")
	}
	if !strings.Contains(out, "Atlassian Platform Connector") {
		t.Error("expected the product name tagline in the logo")
	}
}

// --- T2: Toggle enabled (space) ---

func TestToggleModule_Space(t *testing.T) {
	m := tui.NewModel()

	// T2.1: space on enabled → disabled
	m2 := pressRune(m, ' ')
	if m2.Modules()[0].Enabled {
		t.Error("T2.1: module[0] should be disabled after space")
	}

	// T2.2: space again → re-enabled
	m3 := pressRune(m2, ' ')
	if !m3.Modules()[0].Enabled {
		t.Error("T2.2: module[0] should be enabled after second space")
	}

	// T2.3: preview updates — disable jira, rest still enabled → not "all"
	m4 := pressRune(m, ' ') // jira disabled
	if m4.Preview() == "all" {
		t.Errorf("T2.3: preview should not be 'all' when jira disabled, got %q", m4.Preview())
	}
}

// --- T3: Toggle access (r) ---

func TestToggleAccess_R(t *testing.T) {
	m := tui.NewModel()

	// T3.1: r on RW module → Read
	m2 := pressRune(m, 'r')
	if m2.Modules()[0].Access != tui.AccessRead {
		t.Errorf("T3.1: access after r: got %d, want AccessRead(%d)", m2.Modules()[0].Access, tui.AccessRead)
	}

	// T3.2: r again → RW
	m3 := pressRune(m2, 'r')
	if m3.Modules()[0].Access != tui.AccessReadWrite {
		t.Errorf("T3.2: access after second r: got %d, want AccessReadWrite(%d)", m3.Modules()[0].Access, tui.AccessReadWrite)
	}

	// T3.3: r on disabled module → no change
	m4 := pressRune(m, ' ')  // disable jira
	m5 := pressRune(m4, 'r') // r on disabled
	if m5.Modules()[0].Access != tui.AccessReadWrite {
		t.Errorf("T3.3: r on disabled module changed access: got %d", m5.Modules()[0].Access)
	}
}

// --- T4: Navigation ---

func TestNavigation(t *testing.T) {
	m := tui.NewModel()

	// T4.3: down increments
	m2 := pressKey(m, tea.KeyDown)
	if m2.Cursor() != 1 {
		t.Errorf("T4.3: cursor after down: got %d, want 1", m2.Cursor())
	}

	// T4.1: down on last → wraps to 0 (9 modules → 9 downs from 0 wraps back)
	mLast := m
	for i := 0; i < 9; i++ {
		mLast = pressKey(mLast, tea.KeyDown)
	}
	if mLast.Cursor() != 0 {
		t.Errorf("T4.1: cursor should wrap to 0, got %d", mLast.Cursor())
	}

	// T4.2: up on first → wraps to last (8 = index of confluence)
	m3 := pressKey(m, tea.KeyUp)
	if m3.Cursor() != 8 {
		t.Errorf("T4.2: cursor after up from 0: got %d, want 8", m3.Cursor())
	}

	// j/k aliases
	mj := pressRune(m, 'j')
	if mj.Cursor() != 1 {
		t.Errorf("j: cursor: got %d, want 1", mj.Cursor())
	}
	mk := pressRune(m, 'k')
	if mk.Cursor() != 8 {
		t.Errorf("k from 0: cursor: got %d, want 8", mk.Cursor())
	}
}

// --- T5: Toggle all (a) ---

func TestToggleAll_A(t *testing.T) {
	m := tui.NewModel()
	// Initial state: 8 modules enabled, confluence disabled → allEnabled=false

	// T5.1: a when any disabled (confluence) → all enabled RW including confluence
	m2 := pressRune(m, 'a')
	for i, mod := range m2.Modules() {
		if !mod.Enabled {
			t.Errorf("T5.1: modules[%d] should be enabled after 'a' (was not all enabled)", i)
		}
		if mod.Access != tui.AccessReadWrite {
			t.Errorf("T5.1: modules[%d].Access: got %d, want RW", i, mod.Access)
		}
	}
	if m2.Preview() != "all" {
		t.Errorf("T5.1: preview after enabling all: got %q, want 'all'", m2.Preview())
	}

	// T5.2: a when all enabled → all disabled
	m3 := pressRune(m2, 'a')
	for i, mod := range m3.Modules() {
		if mod.Enabled {
			t.Errorf("T5.2: modules[%d] should be disabled after 'a' (was all enabled)", i)
		}
	}
	if m3.Preview() != "" {
		t.Errorf("T5.2: preview after disabling all: got %q, want ''", m3.Preview())
	}

	// T5.3: a when all disabled → all enabled RW again
	m4 := pressRune(m3, 'a')
	for i, mod := range m4.Modules() {
		if !mod.Enabled {
			t.Errorf("T5.3: modules[%d] should be enabled after 'a'", i)
		}
	}
	if m4.Preview() != "all" {
		t.Errorf("T5.3: preview after re-enable all: got %q, want 'all'", m4.Preview())
	}
}

// helper: advance model through all screens to ScreenRegister using 's' to skip creds/test
func advanceToRegister(m tui.Model) tui.Model {
	// modules → credentials (enter)
	m = pressKey(m, tea.KeyEnter)
	// credentials → test (s = skip — starts async connectivity check)
	m = pressRune(m, 's')
	// Simulate the async testConnMsg completing (all modules pass)
	m = m.SimulateTestResults([]tui.TestResult{
		{Module: "jira", OK: true, Message: "authenticated"},
	})
	// test screen done → register (enter)
	m = pressKey(m, tea.KeyEnter)
	return m
}

// --- T6: Enter advances through screens ---

func TestEnterAdvancesScreen(t *testing.T) {
	m := tui.NewModel()

	// T6.1: enter on modules → ScreenCredentials
	m2 := pressKey(m, tea.KeyEnter)
	if m2.Screen() != tui.ScreenCredentials {
		t.Errorf("T6.1: screen after enter on modules: got %d, want ScreenCredentials(%d)", m2.Screen(), tui.ScreenCredentials)
	}

	// T6.2: 's' on credentials → ScreenTest
	m3 := pressRune(m2, 's')
	if m3.Screen() != tui.ScreenTest {
		t.Errorf("T6.2: screen after s on credentials: got %d, want ScreenTest(%d)", m3.Screen(), tui.ScreenTest)
	}

	// Simulate async connectivity tests completing
	m3 = m3.SimulateTestResults([]tui.TestResult{
		{Module: "jira", OK: true, Message: "authenticated"},
	})

	// T6.3: enter on test → ScreenRegister
	m4 := pressKey(m3, tea.KeyEnter)
	if m4.Screen() != tui.ScreenRegister {
		t.Errorf("T6.3: screen after enter on test: got %d, want ScreenRegister(%d)", m4.Screen(), tui.ScreenRegister)
	}

	// T6.4: register options exist
	opts := m4.RegOpts()
	if len(opts) < 4 {
		t.Errorf("T6.4: expected >=4 register options, got %d", len(opts))
	}

	// T6.5: regCursor starts at 0
	if m4.RegCursor() != 0 {
		t.Errorf("T6.5: regCursor: got %d, want 0", m4.RegCursor())
	}
}

// --- T7: Register screen navigation ---

func TestRegisterNav(t *testing.T) {
	m := advanceToRegister(tui.NewModel())

	// T7.1: down moves regCursor
	m2 := pressKey(m, tea.KeyDown)
	if m2.RegCursor() != 1 {
		t.Errorf("T7.1: regCursor after down: got %d, want 1", m2.RegCursor())
	}

	// wraps at bottom
	mBottom := m
	for i := 0; i < len(m.RegOpts()); i++ {
		mBottom = pressKey(mBottom, tea.KeyDown)
	}
	if mBottom.RegCursor() != 0 {
		t.Errorf("T7.1: regCursor wrap: got %d, want 0", mBottom.RegCursor())
	}

	// T7.2: up wraps to bottom
	m3 := pressKey(m, tea.KeyUp)
	wantLast := len(m.RegOpts()) - 1
	if m3.RegCursor() != wantLast {
		t.Errorf("T7.2: regCursor after up from 0: got %d, want %d", m3.RegCursor(), wantLast)
	}

	// T7.3: esc → back to ScreenTest
	m4 := pressKey(m, tea.KeyEsc)
	if m4.Screen() != tui.ScreenTest {
		t.Errorf("T7.3: esc from register should go to ScreenTest, got %d", m4.Screen())
	}
}

// --- T8: Register enter → done screen ---

func TestRegisterEnter(t *testing.T) {
	m := advanceToRegister(tui.NewModel())

	// Navigate to "Skip" (last option)
	skipIdx := len(m.RegOpts()) - 1
	for i := 0; i < skipIdx; i++ {
		m = pressKey(m, tea.KeyDown)
	}

	// T8.1: enter on Skip → ScreenDone, no panic
	m2 := pressKey(m, tea.KeyEnter)
	if m2.Screen() != tui.ScreenDone {
		t.Errorf("T8.1: screen after Skip enter: got %d, want ScreenDone(%d)", m2.Screen(), tui.ScreenDone)
	}

	// T8.3: doneMsg contains the command string
	if !strings.Contains(m2.DoneMsg(), "atlassian-mcp mcp --enable") {
		t.Errorf("T8.3: doneMsg should contain command, got %q", m2.DoneMsg())
	}
}

// --- T9: Quit ---

func TestQuit(t *testing.T) {
	m := tui.NewModel()

	// T9.1: q → quit cmd returned
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("T9.1: q should return non-nil quit cmd")
	}

	// T9.2: ctrl+c
	_, cmd2 := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd2 == nil {
		t.Error("T9.2: ctrl+c should return non-nil quit cmd")
	}
}

// disableAll is a helper that returns a model with every module disabled.
// Since initial state has confluence disabled (lean default), first 'a'
// enables all (allEnabled=false branch), second 'a' disables all.
func disableAll(m tui.Model) tui.Model {
	m = pressRune(m, 'a') // enable all (confluence was off → allEnabled=false)
	m = pressRune(m, 'a') // disable all (all now on → allEnabled=true)
	return m
}

// --- T10: Preview string ---

func TestPreviewString(t *testing.T) {
	m := tui.NewModel()

	// T10.1: initial state (confluence off by default) → lean list, not "all"
	wantLeanPreview := "jira,agile,goals,metrics,releases,projects,teams,bitbucket"
	if m.Preview() != wantLeanPreview {
		t.Errorf("T10.1: initial lean state → got %q, want %q", m.Preview(), wantLeanPreview)
	}

	// T10.2: only jira enabled RW → "jira"
	m2 := disableAll(m)     // all disabled
	m2 = pressRune(m2, ' ') // enable jira (cursor=0)
	if m2.Preview() != "jira" {
		t.Errorf("T10.2: only jira → got %q, want 'jira'", m2.Preview())
	}

	// T10.3: jira-read + agile RW → "jira-read,agile"
	m3 := disableAll(m)
	m3 = pressRune(m3, ' ')        // enable jira (cursor=0)
	m3 = pressRune(m3, 'r')        // jira → read-only
	m3 = pressKey(m3, tea.KeyDown) // cursor → 1 (agile)
	m3 = pressRune(m3, ' ')        // enable agile
	if m3.Preview() != "jira-read,agile" {
		t.Errorf("T10.3: jira-read,agile → got %q, want 'jira-read,agile'", m3.Preview())
	}

	// T10.4: no modules enabled → ""
	m4 := disableAll(m)
	if m4.Preview() != "" {
		t.Errorf("T10.4: none enabled → got %q, want ''", m4.Preview())
	}

	// T10.5: preview follows allModules order (jira before agile)
	m5b := disableAll(m)
	m5b = pressKey(m5b, tea.KeyDown) // → cursor=1 (agile)
	m5b = pressRune(m5b, ' ')        // enable agile
	m5b = pressKey(m5b, tea.KeyUp)   // → cursor=0 (jira)
	m5b = pressRune(m5b, ' ')        // enable jira
	// preview should be "jira,agile" not "agile,jira"
	if m5b.Preview() != "jira,agile" {
		t.Errorf("T10.5: order — got %q, want 'jira,agile'", m5b.Preview())
	}
}

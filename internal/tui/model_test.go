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

	// T1.4 preview = "all"
	if m.Preview() != "all" {
		t.Errorf("Preview: got %q, want 'all'", m.Preview())
	}

	// T1.2 + T1.5 all 7 modules enabled RW in order
	wantModules := []string{"jira", "agile", "goals", "metrics", "releases", "projects", "teams"}
	mods := m.Modules()
	if len(mods) != len(wantModules) {
		t.Fatalf("Modules len: got %d, want %d", len(mods), len(wantModules))
	}
	for i, mod := range mods {
		if mod.Name != wantModules[i] {
			t.Errorf("modules[%d].Name: got %q, want %q", i, mod.Name, wantModules[i])
		}
		if !mod.Enabled {
			t.Errorf("modules[%d].Enabled: got false, want true", i)
		}
		if mod.Access != tui.AccessReadWrite {
			t.Errorf("modules[%d].Access: got %d, want AccessReadWrite", i, mod.Access)
		}
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

	// T4.1: down on last → wraps to 0
	mLast := m
	for i := 0; i < 7; i++ {
		mLast = pressKey(mLast, tea.KeyDown)
	}
	if mLast.Cursor() != 0 {
		t.Errorf("T4.1: cursor should wrap to 0, got %d", mLast.Cursor())
	}

	// T4.2: up on first → wraps to last (6)
	m3 := pressKey(m, tea.KeyUp)
	if m3.Cursor() != 6 {
		t.Errorf("T4.2: cursor after up from 0: got %d, want 6", m3.Cursor())
	}

	// j/k aliases
	mj := pressRune(m, 'j')
	if mj.Cursor() != 1 {
		t.Errorf("j: cursor: got %d, want 1", mj.Cursor())
	}
	mk := pressRune(m, 'k')
	if mk.Cursor() != 6 {
		t.Errorf("k from 0: cursor: got %d, want 6", mk.Cursor())
	}
}

// --- T5: Toggle all (a) ---

func TestToggleAll_A(t *testing.T) {
	m := tui.NewModel()

	// T5.1: a when all enabled → all disabled
	m2 := pressRune(m, 'a')
	for i, mod := range m2.Modules() {
		if mod.Enabled {
			t.Errorf("T5.1: modules[%d] should be disabled after 'a'", i)
		}
	}

	// T5.2: a when any disabled → all enabled RW
	m3 := pressRune(m2, 'a')
	for i, mod := range m3.Modules() {
		if !mod.Enabled {
			t.Errorf("T5.2: modules[%d] should be enabled after second 'a'", i)
		}
		if mod.Access != tui.AccessReadWrite {
			t.Errorf("T5.2: modules[%d].Access: got %d, want RW", i, mod.Access)
		}
	}
	if m3.Preview() != "all" {
		t.Errorf("T5.2: preview after re-enable all: got %q, want 'all'", m3.Preview())
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

// --- T10: Preview string ---

func TestPreviewString(t *testing.T) {
	m := tui.NewModel()

	// T10.1: all enabled RW → "all"
	if m.Preview() != "all" {
		t.Errorf("T10.1: all enabled → got %q, want 'all'", m.Preview())
	}

	// T10.2: only jira enabled RW → "jira"
	// disable all others
	m2 := pressRune(m, 'a') // disable all
	// enable jira only: cursor is at 0 (jira)
	m2 = pressRune(m2, ' ') // enable jira
	if m2.Preview() != "jira" {
		t.Errorf("T10.2: only jira → got %q, want 'jira'", m2.Preview())
	}

	// T10.3: jira-read + agile RW → "jira-read,agile"
	m3 := pressRune(m, 'a') // disable all
	m3 = pressRune(m3, ' ') // enable jira (cursor=0)
	m3 = pressRune(m3, 'r') // jira → read-only
	m3 = pressKey(m3, tea.KeyDown)  // cursor → 1 (agile)
	m3 = pressRune(m3, ' ') // enable agile
	if m3.Preview() != "jira-read,agile" {
		t.Errorf("T10.3: jira-read,agile → got %q, want 'jira-read,agile'", m3.Preview())
	}

	// T10.4: no modules enabled → ""
	m4 := pressRune(m, 'a') // disable all
	if m4.Preview() != "" {
		t.Errorf("T10.4: none enabled → got %q, want ''", m4.Preview())
	}

	// T10.5: preview follows allModules order (jira before agile)
	m5 := pressRune(m, 'a') // disable all
	// enable agile first (cursor at 0=jira, skip to 1=agile)
	m5 = pressKey(m5, tea.KeyDown)
	m5 = pressRune(m5, ' ') // enable agile
	m5 = pressKey(m5, tea.KeyUp) // back to jira
	m5 = pressKey(m5, tea.KeyUp) // cursor wraps... let's go back to 0
	// simpler: navigate to 0
	// Actually cursor is at 1 after down. Up from 1 → 0.
	m5b := pressRune(m, 'a') // disable all
	m5b = pressKey(m5b, tea.KeyDown) // → cursor=1 (agile)
	m5b = pressRune(m5b, ' ')        // enable agile
	m5b = pressKey(m5b, tea.KeyUp)   // → cursor=0 (jira)
	m5b = pressRune(m5b, ' ')        // enable jira
	// preview should be "jira,agile" not "agile,jira"
	if m5b.Preview() != "jira,agile" {
		t.Errorf("T10.5: order — got %q, want 'jira,agile'", m5b.Preview())
	}
}

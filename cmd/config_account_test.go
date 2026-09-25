package cmd

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/suho-han/one-click-ai-tools/internal/update"
	"github.com/suho-han/one-click-ai-tools/internal/usage"
)

func TestConfigAccountCommands(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)
	viper.SetConfigFile(filepath.Join(tmp, ".oct", "config.yaml"))
	t.Cleanup(func() {
		viper.Set("accounts", nil)
		viper.Set("codex_accounts", nil)
		viper.SetConfigFile("")
	})

	var out bytes.Buffer

	// add persists the documented provider-keyed shape and reports the row name.
	if err := configAccountAddCmd.RunE(configAccountAddCmd, []string{"Codex", "Work", "~/codex-work"}); err != nil {
		t.Fatalf("add: %v", err)
	}
	entries := usage.AccountConfigEntries()
	if len(entries) != 1 || entries[0].Provider != "codex" || entries[0].Name != "work" || entries[0].Home != "~/codex-work" {
		t.Fatalf("entries after add = %+v", entries)
	}

	// add with an existing provider+name updates the home instead of duplicating.
	if err := configAccountAddCmd.RunE(configAccountAddCmd, []string{"codex", "work", "~/codex-work-2"}); err != nil {
		t.Fatalf("add (update): %v", err)
	}
	entries = usage.AccountConfigEntries()
	if len(entries) != 1 || entries[0].Home != "~/codex-work-2" {
		t.Fatalf("entries after re-add = %+v, want updated home", entries)
	}

	// Unsupported providers are rejected up front.
	if err := configAccountAddCmd.RunE(configAccountAddCmd, []string{"claude", "work", "~/claude-work"}); err == nil {
		t.Fatal("add for an unsupported provider should fail")
	}

	// The config file actually holds the entry (written via writeConfig).
	data, err := os.ReadFile(filepath.Join(tmp, ".oct", "config.yaml"))
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}
	if !bytes.Contains(data, []byte("accounts:")) || !bytes.Contains(data, []byte("codex-work-2")) {
		t.Fatalf("written config missing accounts entry:\n%s", data)
	}

	// list renders one line per account with the resolved path.
	viper.Set("accounts", map[string][]map[string]string{
		"codex": {{"name": "work", "home": "~/codex-work-2"}},
		"kimi":  {{"name": "personal", "home": filepath.Join(tmp, "kimi-personal")}},
	})
	configAccountListCmd.SetOut(&out)
	if err := configAccountListCmd.RunE(configAccountListCmd, nil); err != nil {
		t.Fatalf("list: %v", err)
	}
	listed := out.String()
	if !bytes.Contains([]byte(listed), []byte("codex:work")) || !bytes.Contains([]byte(listed), []byte(filepath.Join(tmp, "codex-work-2"))) {
		t.Fatalf("list output missing codex:work row: %q", listed)
	}
	if !bytes.Contains([]byte(listed), []byte("kimi:personal")) {
		t.Fatalf("list output missing kimi:personal row: %q", listed)
	}

	// remove deletes the named account and reports unknown names.
	out.Reset()
	if err := configAccountRemoveCmd.RunE(configAccountRemoveCmd, []string{"codex", "work"}); err != nil {
		t.Fatalf("remove: %v", err)
	}
	entries = usage.AccountConfigEntries()
	if len(entries) != 1 || entries[0].Provider != "kimi" {
		t.Fatalf("entries after remove = %+v, want only kimi:personal", entries)
	}
	if err := configAccountRemoveCmd.RunE(configAccountRemoveCmd, []string{"codex", "ghost"}); err == nil {
		t.Fatal("remove of unknown account should fail")
	}

	// upsertAccount is shared with the TUI add-account flow.
	replaced, err := upsertAccount("codex", "work", "~/codex-tui")
	if err != nil {
		t.Fatalf("upsertAccount: %v", err)
	}
	if replaced {
		t.Fatal("upsertAccount reported replaced for a fresh entry")
	}
	replaced, err = upsertAccount("codex", "work", "~/codex-tui-2")
	if err != nil || !replaced {
		t.Fatalf("upsertAccount update = %v,%v, want true,nil", replaced, err)
	}
	accounts := usage.AccountsForProvider("codex")
	if len(accounts) != 1 || accounts[0].Home != filepath.Join(tmp, "codex-tui-2") {
		t.Fatalf("accounts after upsert = %+v", accounts)
	}
}

// stubLogin captures the login spawn instead of running codex and writes an
// auth.json with the given id_token, simulating a successful browser login.
func stubLogin(t *testing.T, idToken string) *string {
	t.Helper()
	orig := runInteractiveLogin
	var loginHome string
	runInteractiveLogin = func(home string) error {
		loginHome = home
		auth := fmt.Sprintf(`{"tokens":{"id_token":%q}}`, idToken)
		return os.WriteFile(filepath.Join(home, "auth.json"), []byte(auth), 0o600)
	}
	t.Cleanup(func() { runInteractiveLogin = orig })
	return &loginHome
}

// fakeJWT builds a syntactically valid unsigned JWT carrying the claims.
func fakeJWT(claims map[string]string) string {
	encode := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	payload, err := json.Marshal(claims)
	if err != nil {
		panic(err)
	}
	return encode(`{"alg":"none"}`) + "." + encode(string(payload)) + ".sig"
}

func TestRunAddCodexAccountFlow_AutoRegistersFromEmail(t *testing.T) {
	tmp := t.TempDir()
	isolateTestHome(t, tmp)
	viper.SetConfigFile(filepath.Join(tmp, ".oct", "config.yaml"))
	t.Cleanup(func() {
		viper.Set("accounts", nil)
		viper.Set("codex_accounts", nil)
		viper.SetConfigFile("")
	})

	// No stdin: the flow must not ask anything when the token has an email.
	loginHome := stubLogin(t, fakeJWT(map[string]string{"email": "hansuho36@gmail.com"}))

	var out bytes.Buffer
	flowCmd := &cobra.Command{}
	flowCmd.SetOut(&out)
	if err := runAddCodexAccountFlow(flowCmd); err != nil {
		t.Fatalf("flow: %v", err)
	}

	listed := out.String()
	if !strings.Contains(listed, "Codex accounts:") || !strings.Contains(listed, "(none registered yet)") {
		t.Fatalf("output missing account list header: %q", listed)
	}
	if !strings.Contains(listed, "✓ Logged in as hansuho36@gmail.com") {
		t.Fatalf("output missing login report: %q", listed)
	}
	// The alias is the masked email prefix; the home uses the raw prefix.
	accounts := usage.AccountsForProvider("codex")
	if len(accounts) != 1 || accounts[0].Name != "han***o36" {
		t.Fatalf("accounts after flow = %+v, want codex:han***o36", accounts)
	}
	if accounts[0].Home != filepath.Join(tmp, ".codex-hansuho36") {
		t.Fatalf("home = %q, want %q", accounts[0].Home, filepath.Join(tmp, ".codex-hansuho36"))
	}
	if _, err := os.Stat(accounts[0].Home); err != nil {
		t.Fatalf("renamed home missing: %v", err)
	}
	if *loginHome == "" {
		t.Fatal("login runner was not called")
	}
	// The temporary login home must be gone after the rename.
	if leftovers, _ := filepath.Glob(filepath.Join(tmp, ".codex-tmp-login-*")); len(leftovers) != 0 {
		t.Fatalf("temporary login homes left behind: %v", leftovers)
	}
}

func TestRunAddCodexAccountFlow_LoginFailureCleansUp(t *testing.T) {
	tmp := t.TempDir()
	isolateTestHome(t, tmp)
	viper.SetConfigFile(filepath.Join(tmp, ".oct", "config.yaml"))
	t.Cleanup(func() {
		viper.Set("accounts", nil)
		viper.Set("codex_accounts", nil)
		viper.SetConfigFile("")
	})

	orig := runInteractiveLogin
	runInteractiveLogin = func(home string) error { return errors.New("browser closed") }
	t.Cleanup(func() { runInteractiveLogin = orig })

	var out bytes.Buffer
	flowCmd := &cobra.Command{}
	flowCmd.SetOut(&out)
	if err := runAddCodexAccountFlow(flowCmd); err != nil {
		t.Fatalf("flow: %v", err)
	}

	if len(usage.AccountsForProvider("codex")) != 0 {
		t.Fatal("failed login must not register an account")
	}
	if !strings.Contains(out.String(), "Login did not complete") {
		t.Fatalf("output missing failure report: %q", out.String())
	}
	if leftovers, _ := filepath.Glob(filepath.Join(tmp, ".codex-tmp-login-*")); len(leftovers) != 0 {
		t.Fatalf("temporary login homes left behind: %v", leftovers)
	}
}

func TestRunAddCodexAccountFlow_DuplicateAliasCleansUp(t *testing.T) {
	tmp := t.TempDir()
	isolateTestHome(t, tmp)
	viper.SetConfigFile(filepath.Join(tmp, ".oct", "config.yaml"))
	viper.Set("accounts", map[string][]map[string]string{
		"codex": {{"name": "han***o36", "home": filepath.Join(tmp, ".codex-hansuho36")}},
	})
	t.Cleanup(func() {
		viper.Set("accounts", nil)
		viper.Set("codex_accounts", nil)
		viper.SetConfigFile("")
	})

	stubLogin(t, fakeJWT(map[string]string{"email": "hansuho36@gmail.com"}))

	var out bytes.Buffer
	flowCmd := &cobra.Command{}
	flowCmd.SetOut(&out)
	if err := runAddCodexAccountFlow(flowCmd); err != nil {
		t.Fatalf("flow: %v", err)
	}

	if !strings.Contains(out.String(), "already registered") {
		t.Fatalf("output missing already-registered report: %q", out.String())
	}
	accounts := usage.AccountsForProvider("codex")
	if len(accounts) != 1 || accounts[0].Home != filepath.Join(tmp, ".codex-hansuho36") {
		t.Fatalf("existing account was modified: %+v", accounts)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(tmp, ".codex-tmp-login-*")); len(leftovers) != 0 {
		t.Fatalf("temporary login homes left behind: %v", leftovers)
	}
}

func TestRunAddCodexAccountFlow_EmailFallbackPrompts(t *testing.T) {
	tmp := t.TempDir()
	isolateTestHome(t, tmp)
	viper.SetConfigFile(filepath.Join(tmp, ".oct", "config.yaml"))
	t.Cleanup(func() {
		viper.Set("accounts", nil)
		viper.Set("codex_accounts", nil)
		viper.SetConfigFile("")
	})

	// Token without an email claim: the flow falls back to a name prompt.
	stubLogin(t, fakeJWT(map[string]string{"sub": "no-email-here"}))
	orig := configPromptReader
	defer func() { configPromptReader = orig }()
	configPromptReader = bufio.NewReader(strings.NewReader("myname\n"))

	var out bytes.Buffer
	flowCmd := &cobra.Command{}
	flowCmd.SetOut(&out)
	if err := runAddCodexAccountFlow(flowCmd); err != nil {
		t.Fatalf("flow: %v", err)
	}

	accounts := usage.AccountsForProvider("codex")
	if len(accounts) != 1 || accounts[0].Name != "myname" || accounts[0].Home != filepath.Join(tmp, ".codex-myname") {
		t.Fatalf("accounts after fallback = %+v, want myname -> %s", accounts, filepath.Join(tmp, ".codex-myname"))
	}
}

func TestMaskEmailPrefix(t *testing.T) {
	if got := maskEmailPrefix("hansuho36"); got != "han***o36" {
		t.Fatalf("maskEmailPrefix(hansuho36) = %q, want han***o36", got)
	}
	if got := maskEmailPrefix("john.doe+work"); got != "joh***ork" {
		t.Fatalf("maskEmailPrefix(john.doe+work) = %q, want joh***ork (non-alnum stripped before masking)", got)
	}
	if got := maskEmailPrefix("abcd"); got != "a***d" {
		t.Fatalf("maskEmailPrefix(abcd) = %q, want a***d", got)
	}
	if got := maskEmailPrefix("abc"); got != "***" {
		t.Fatalf("maskEmailPrefix(abc) = %q, want ***", got)
	}
	if got := maskEmailPrefix(""); got != "***" {
		t.Fatalf("maskEmailPrefix(empty) = %q, want ***", got)
	}
}

func TestConfigAccountAddDerivesHomeFromName(t *testing.T) {
	tmp := t.TempDir()
	isolateTestHome(t, tmp)
	viper.SetConfigFile(filepath.Join(tmp, ".oct", "config.yaml"))
	t.Cleanup(func() {
		viper.Set("accounts", nil)
		viper.Set("codex_accounts", nil)
		viper.SetConfigFile("")
	})

	// Two-argument form derives ~/.<provider>-<name>.
	if err := configAccountAddCmd.RunE(configAccountAddCmd, []string{"kimi", "work"}); err != nil {
		t.Fatalf("add: %v", err)
	}
	entries := usage.AccountConfigEntries()
	if len(entries) != 1 || entries[0].Home != "~/.kimi-work" {
		t.Fatalf("entries = %+v, want kimi/work with derived ~/.kimi-work", entries)
	}
}

// TestConfigAccountCommandsPreserveTokenFields pins that key/env (the
// reserved token-kind fields) survive round-trips through the management
// commands — dropping them would silently erase hand-edited entries.
func TestConfigAccountCommandsPreserveTokenFields(t *testing.T) {
	tmp := t.TempDir()
	isolateTestHome(t, tmp)
	viper.SetConfigFile(filepath.Join(tmp, ".oct", "config.yaml"))
	viper.Set("accounts", map[string][]map[string]string{
		"codex": {{"name": "tok", "home": "~/tok-home", "key": "sk-secret", "env": "TOK_KEY"}},
	})
	t.Cleanup(func() {
		viper.Set("accounts", nil)
		viper.Set("codex_accounts", nil)
		viper.SetConfigFile("")
	})

	// An unrelated add must not disturb the key/env fields of tok.
	if err := configAccountAddCmd.RunE(configAccountAddCmd, []string{"codex", "other", "~/other-home"}); err != nil {
		t.Fatalf("add: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(tmp, ".oct", "config.yaml"))
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}
	for _, want := range []string{"sk-secret", "TOK_KEY"} {
		if !bytes.Contains(data, []byte(want)) {
			t.Fatalf("written config dropped reserved field %q:\n%s", want, data)
		}
	}
	entries := usage.AccountConfigEntries()
	var tok *usage.AccountEntry
	for i := range entries {
		if entries[i].Name == "tok" {
			tok = &entries[i]
		}
	}
	if tok == nil || tok.Key != "sk-secret" || tok.Env != "TOK_KEY" {
		t.Fatalf("tok entry = %+v, want key/env preserved", tok)
	}
}

func TestConfigModel_ManageAccountsRowUnderCodex(t *testing.T) {
	tmp := t.TempDir()
	isolateTestHome(t, tmp)
	viper.Set("accounts", map[string][]map[string]string{
		"codex": {
			{"name": "han***o36", "home": filepath.Join(tmp, ".codex-hansuho36")},
			{"name": "ghost", "home": filepath.Join(tmp, ".codex-ghost")},
		},
	})
	t.Cleanup(func() { viper.Set("accounts", nil) })

	m := newConfigModel([]string{"codex"}, []string{"codex"})
	codexIdx, addIdx, manageIdx := -1, -1, -1
	for i, it := range m.items {
		switch {
		case update.NormalizeToolName(it.tool.BinaryName) == "codex":
			codexIdx = i
		case it.isAddAccountControl:
			addIdx = i
		case it.isManageAccountsRow:
			manageIdx = i
		}
	}
	if codexIdx < 0 || addIdx != codexIdx+1 || manageIdx != addIdx+1 {
		t.Fatalf("codex group order wrong: codex=%d add=%d manage=%d, want add then manage right after codex", codexIdx, addIdx, manageIdx)
	}
	if m.items[manageIdx].manageCount != 2 {
		t.Fatalf("manageCount = %d, want 2", m.items[manageIdx].manageCount)
	}
	// Collapsed by default: no account rows in the list.
	for _, it := range m.items {
		if it.isAccountRow || it.isAccountActionRow {
			t.Fatalf("account rows must be hidden while collapsed: %+v", it)
		}
	}
	// The manage row renders as a label with the count, not a checkbox.
	line := m.renderItemLines(m.items[manageIdx])[0]
	if !strings.Contains(line, "Manage existing accounts (2)") || strings.Contains(line, "[ ]") {
		t.Fatalf("manage row = %q, want counted label without checkbox", line)
	}

	// Enter expands the group: account rows appear below it.
	for i := range m.items {
		m.items[i].cursor = i == manageIdx
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(configModel)
	if !mm.items[manageIdx+1].isAccountRow || mm.items[manageIdx+1].accountAlias != "han***o36" {
		t.Fatalf("first expanded row = %+v, want han***o36", mm.items[manageIdx+1])
	}
	if !mm.items[manageIdx+2].isAccountRow || mm.items[manageIdx+2].accountAlias != "ghost" {
		t.Fatalf("second expanded row = %+v, want ghost", mm.items[manageIdx+2])
	}
	row := mm.renderItemLines(mm.items[manageIdx+1])[0]
	if !strings.Contains(row, "codex:han***o36") || !strings.Contains(row, "(missing)") {
		t.Fatalf("account row = %q, want masked label with (missing)", row)
	}

	// Enter again collapses the group.
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(configModel)
	for _, it := range m2.items {
		if it.isAccountRow || it.isAccountActionRow {
			t.Fatalf("account rows must hide after collapsing: %+v", it)
		}
	}
}

func TestConfigModel_NoManageRowWithoutAccounts(t *testing.T) {
	m := newConfigModel([]string{"codex"}, []string{"codex"})
	for _, it := range m.items {
		if it.isManageAccountsRow || it.isAccountRow {
			t.Fatalf("account UI must be absent without registered accounts: %+v", it)
		}
	}
}

func TestConfigModel_ExpandAccountShowsActions(t *testing.T) {
	viper.Set("accounts", map[string][]map[string]string{
		"codex": {{"name": "work", "home": "/tmp/.codex-work"}},
	})
	t.Cleanup(func() { viper.Set("accounts", nil) })

	m := newConfigModel([]string{"codex"}, []string{"codex"})
	manageIdx := -1
	for i, it := range m.items {
		if it.isManageAccountsRow {
			manageIdx = i
		}
	}
	if manageIdx < 0 {
		t.Fatal("manage row missing although an account is registered")
	}
	// Expand the manage group first.
	for i := range m.items {
		m.items[i].cursor = i == manageIdx
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm := updated.(configModel)
	accountIdx := -1
	for i, it := range mm.items {
		if it.isAccountRow {
			accountIdx = i
		}
	}
	if accountIdx < 0 {
		t.Fatalf("account row missing after expanding manage group: %+v", mm.items)
	}

	// Enter on the account row expands its actions.
	for i := range mm.items {
		mm.items[i].cursor = i == accountIdx
	}
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mm = updated.(configModel)
	if mm.items[accountIdx+1].accountActionKind != "reconnect" || mm.items[accountIdx+2].accountActionKind != "disconnect" {
		t.Fatalf("expanded rows = %+v, %+v, want reconnect then disconnect", mm.items[accountIdx+1], mm.items[accountIdx+2])
	}

	// Enter on the reconnect action quits with a reconnect request.
	for i := range mm.items {
		mm.items[i].cursor = i == accountIdx+1
	}
	updated, quit := mm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(configModel)
	if !m2.done || m2.accountAction == nil || m2.accountAction.kind != "reconnect" || m2.accountAction.alias != "work" {
		t.Fatalf("action = %+v done=%v, want reconnect/work", m2.accountAction, m2.done)
	}
	if quit == nil {
		t.Fatal("expected quit command")
	}
}

func TestRunReconnectCodexAccountFlow(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, ".codex-work")

	orig := runInteractiveLogin
	var got string
	runInteractiveLogin = func(home string) error {
		got = home
		return os.WriteFile(filepath.Join(home, "auth.json"), []byte(`{"tokens":{}}`), 0o600)
	}
	t.Cleanup(func() { runInteractiveLogin = orig })

	var out bytes.Buffer
	flowCmd := &cobra.Command{}
	flowCmd.SetOut(&out)
	if err := runReconnectCodexAccountFlow(flowCmd, "work", home); err != nil {
		t.Fatalf("flow: %v", err)
	}
	if got != home {
		t.Fatalf("login home = %q, want %q", got, home)
	}
	if !strings.Contains(out.String(), "✓ Reconnected codex:work") {
		t.Fatalf("output missing reconnect confirmation: %q", out.String())
	}
}

func TestRunDisconnectCodexAccountFlow(t *testing.T) {
	tmp := t.TempDir()
	isolateTestHome(t, tmp)
	viper.SetConfigFile(filepath.Join(tmp, ".oct", "config.yaml"))
	viper.Set("accounts", map[string][]map[string]string{
		"codex": {
			{"name": "work", "home": "~/codex-work"},
			{"name": "personal", "home": "~/codex-personal"},
		},
	})
	t.Cleanup(func() {
		viper.Set("accounts", nil)
		viper.Set("codex_accounts", nil)
		viper.SetConfigFile("")
	})

	orig := configPromptReader
	defer func() { configPromptReader = orig }()

	// Declining keeps the account.
	configPromptReader = bufio.NewReader(strings.NewReader("n\n"))
	var out bytes.Buffer
	flowCmd := &cobra.Command{}
	flowCmd.SetOut(&out)
	if err := runDisconnectCodexAccountFlow(flowCmd, "work"); err != nil {
		t.Fatalf("flow: %v", err)
	}
	if len(usage.AccountsForProvider("codex")) != 2 {
		t.Fatal("declined disconnect must keep the account")
	}

	// Confirming removes only that account.
	configPromptReader = bufio.NewReader(strings.NewReader("y\n"))
	if err := runDisconnectCodexAccountFlow(flowCmd, "work"); err != nil {
		t.Fatalf("flow: %v", err)
	}
	accounts := usage.AccountsForProvider("codex")
	if len(accounts) != 1 || accounts[0].Name != "personal" {
		t.Fatalf("accounts after disconnect = %+v, want only personal", accounts)
	}
}

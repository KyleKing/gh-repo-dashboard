//nolint:testpackage // drives Model.Update directly like the rest of this package's tests
package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/keyusage"
)

func TestKeyUsageLogsCommandKeysPerContextAndNeverTypedText(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	rec := keyusage.Open(UsageApp, keyusage.WithDir(dir), keyusage.WithHost("test"))
	m := newListModel(testRepo1Path).WithUsage(rec, nil)

	for _, k := range []tea.KeyPressMsg{
		keyPress('f'), keyPress('*'),
		{Code: tea.KeyEscape},
		keyPress('/'), keyPress('x'), keyPress('a'), keyPress('x'),
		{Code: tea.KeyEscape},
		keyPress('x'),
	} {
		next, _ := m.Update(k)
		m = mustModel(t, next)
	}

	if err := rec.Close(); err != nil {
		t.Fatal(err)
	}

	u, err := keyusage.Load(dir, Keymap())
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []struct {
		ctx, path string
		count     int
	}{
		{ctxRepoList, "f", 1},
		{ctxFilter, "*", 1},
		{ctxRepoList, "/", 1},
		{ctxRepoList, "a", 0},
		{ctxRepoList, "x", 1},
	} {
		if got := u.Stat(want.ctx, want.path).Count; got != want.count {
			t.Errorf("%s %q logged %d times, want %d", want.ctx, want.path, got, want.count)
		}
	}
}

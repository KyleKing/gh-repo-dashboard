//nolint:testpackage // checkGHAuthCmd is unexported; see ROADMAP.md
package app

import (
	"strings"
	"testing"
)

func TestCheckGHAuthCmd_ReportsMissingGH(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	msg, ok := checkGHAuthCmd()().(GHAuthCheckedMsg)
	if !ok {
		t.Fatalf("checkGHAuthCmd returned %T, want GHAuthCheckedMsg", msg)
	}
	if !strings.Contains(msg.Message, "gh not found") {
		t.Errorf("message = %q, want it to name gh missing from PATH", msg.Message)
	}
}

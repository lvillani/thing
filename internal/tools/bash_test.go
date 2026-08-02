// SPDX-License-Identifier: GPL-3.0-only

package tools

import (
	"strings"
	"testing"
)

func TestBash_SuccessReturnsOutput(t *testing.T) {
	out, err := (&bash{}).Run(`{"command":"echo hello"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(out) != "hello" {
		t.Errorf("out = %q, want 'hello'", out)
	}
}

func TestBash_NonZeroRcReturnsOutputWithErr(t *testing.T) {
	// A command that exits nonzero but writes to stderr must return an error that
	// carries the output, so the model can see what went wrong.
	out, err := (&bash{}).Run(`{"command":"echo boom >&2; exit 1"}`)
	if err == nil {
		t.Fatal("expected an error for rc=1, got nil")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error %q does not carry the command's stderr output", err.Error())
	}
	if !strings.Contains(err.Error(), "rc=") {
		t.Errorf("error %q does not carry the exit code", err.Error())
	}
	if out != "" {
		t.Errorf("out = %q, want empty (error case)", out)
	}
}

// SPDX-License-Identifier: GPL-3.0-only

package tools

import (
	"context"
	"errors"
	"testing"

	"thing/internal/model"
)

func TestRegistry_UnknownToolReturnsSentinelError(t *testing.T) {
	_, err := NewRegistry().Run(context.Background(), "unknown", model.ToolCallFunctionArguments(`{}`))
	if !errors.Is(err, errToolNotFound) {
		t.Fatalf("error = %v, want errToolNotFound", err)
	}
}

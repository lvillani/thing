// SPDX-License-Identifier: GPL-3.0-only

package statusbar

import (
	"strings"
	"testing"
)

func TestShortenHome(t *testing.T) {
	tests := []struct {
		name      string
		directory string
		home      string
		want      string
	}{
		{
			name:      "directory below home",
			directory: "/home/user/Development/thing",
			home:      "/home/user",
			want:      "~/Development/thing",
		},
		{
			name:      "home directory",
			directory: "/home/user",
			home:      "/home/user",
			want:      "~",
		},
		{
			name:      "directory outside home",
			directory: "/opt/thing",
			home:      "/home/user",
			want:      "/opt/thing",
		},
		{
			name:      "directory with home prefix but outside home",
			directory: "/home/username/thing",
			home:      "/home/user",
			want:      "/home/username/thing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shortenHome(tt.directory, tt.home); got != tt.want {
				t.Errorf("shortenHome(%q, %q) = %q, want %q", tt.directory, tt.home, got, tt.want)
			}
		})
	}
}

func TestViewShowsModel(t *testing.T) {
	m := Model{directory: "~/Development/thing", model: "gpt-4o"}

	if got := m.View(); !strings.Contains(got, "~/Development/thing · gpt-4o") {
		t.Errorf("View() = %q, want directory and model", got)
	}
}

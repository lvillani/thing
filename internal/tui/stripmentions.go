// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// stripMentions removes the leading "@" from file references ("@path/to/file") in
// user input. The "@" is a TUI-only trigger for the file-mention popup, not content;
// the model must receive the bare path so it doesn't try to open a path literally
// prefixed with "@" (mirroring how "/skill:" is resolved before sending). The raw
// input keeps the "@" for echo and history; only the string sent to the model is
// cleaned here.
//
// Only a word-bounded "@" qualifies: it must be preceded by the start of the input
// or a non-alphanumeric, and followed by a non-space. This keeps real at-signs that
// are part of a word (e.g. "a@example.com") untouched.
func stripMentions(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prev := rune(0)
	for i := 0; i < len(s); {
		r, size := rune(s[i]), 1
		if r >= 0x80 {
			r, size = utf8.DecodeRuneInString(s[i:])
		}
		if r == '@' && isWordBoundaryBefore(prev) && !isSpaceAfter(s, i+size) {
			// strip the '@'
		} else {
			b.WriteRune(r)
		}
		prev = r
		i += size
	}
	return b.String()
}

// isWordBoundaryBefore reports whether the character before the "@" is not part of a
// word (start of input, whitespace, or punctuation), so "a@b" is left alone but
// "@path" / "(@path)" are stripped. Underscores count as word characters.
func isWordBoundaryBefore(r rune) bool {
	return r == 0 || unicode.IsSpace(r) || unicode.IsPunct(r)
}

// isSpaceAfter reports whether nothing but whitespace (or the end of input) follows
// the "@"; in those cases the "@" is not a file reference (a bare "@", "@ word", or a
// trailing "@") and should be kept.
func isSpaceAfter(s string, i int) bool {
	return i >= len(s) || unicode.IsSpace(rune(s[i]))
}

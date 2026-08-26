// SPDX-License-Identifier: GPL-3.0-only

package tools

import "errors"

// Generic tool errors.
var (
	errToolBadArguments = errors.New("bad arguments")
	errToolNotFound     = errors.New("tool not found")
)

// Bash tool errors.
var (
	errBashCommandFailed = errors.New("command failed")
)

// Edit tool errors.
var (
	errEditCannotReadFile         = errors.New("cannot read file")
	errEditTextNotFound           = errors.New("text not found")
	errEditTextFoundMultipleTimes = errors.New("text found multiple times")
	errEditCannotWriteFile        = errors.New("cannot write file")
)

// Read tool errors.
var (
	errReadCannotReadFile = errors.New("cannot read file")
)

// Write tool errors.
var (
	errWriteCannotCreateParentDirectories = errors.New("cannot create parent directories")
	errWriteCannotWriteFile               = errors.New("cannot write file")
)

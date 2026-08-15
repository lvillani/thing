// SPDX-License-Identifier: GPL-3.0-only

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"thing/internal/model"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

// glob is a tool that finds files by name and returns them by modification time.
type glob struct{}

// Describe returns the description of the glob tool.
func (g *glob) Describe() model.Tool {
	return model.Tool{
		Type: model.ToolTypeFunction,
		Function: model.ToolFunctionDefinition{
			Name:        "glob",
			Description: "Find files matching a pattern, sorted by modification time (newest first)",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"directory": map[string]any{
						"type":        "string",
						"description": "Directory to search",
					},
					"pattern": map[string]any{
						"type":        "string",
						"description": "File pattern, such as **/*.go",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of paths to return (default: 1000)",
					},
				},
				"required": []string{"directory", "pattern"},
			},
		},
	}
}

// Run finds matching files below directory.
func (g *glob) Run(ctx context.Context, input model.ToolCallFunctionArguments) (string, error) {
	var args struct {
		Directory string `json:"directory"`
		Pattern   string `json:"pattern"`
		Limit     int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(input), &args); err != nil {
		return "", fmt.Errorf("bad arguments: %v", err)
	}

	if args.Directory == "" {
		return "", fmt.Errorf("directory is required")
	}

	if args.Limit == 0 {
		args.Limit = 1000
	}
	if args.Limit < 0 {
		return "", fmt.Errorf("limit must not be negative")
	}

	type match struct {
		path    string
		modTime int64
	}

	patterns, err := gitignore.ReadPatterns(osfs.New(args.Directory), nil)
	if err != nil {
		return "", fmt.Errorf("cannot read gitignore: %v", err)
	}

	gitignoreMatcher := gitignore.NewMatcher(patterns)

	matches := make([]match, 0)
	if err := filepath.WalkDir(args.Directory, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if path == args.Directory {
			return nil
		}

		relative, err := filepath.Rel(args.Directory, path)
		if err != nil {
			return err
		}

		parts := strings.Split(filepath.ToSlash(relative), "/")
		if parts[0] == ".git" || gitignoreMatcher.Match(parts, entry.IsDir()) {
			if entry.IsDir() {
				return filepath.SkipDir
			}

			return nil
		}

		if entry.IsDir() {
			return nil
		}

		matched, err := filepath.Match(args.Pattern, relative)
		if err != nil {
			return fmt.Errorf("invalid pattern: %v", err)
		}

		if !matched {
			matched, err = filepath.Match(args.Pattern, entry.Name())
			if err != nil {
				return fmt.Errorf("invalid pattern: %v", err)
			}
		}

		if !matched {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		matches = append(matches, match{path: path, modTime: info.ModTime().UnixNano()})

		return nil
	}); err != nil {
		return "", fmt.Errorf("cannot search directory: %v", err)
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].modTime != matches[j].modTime {
			return matches[i].modTime > matches[j].modTime
		}

		return matches[i].path < matches[j].path
	})

	if len(matches) > args.Limit {
		matches = matches[:args.Limit]
	}

	result := make([]string, len(matches))
	for i, item := range matches {
		result[i] = item.path
	}

	return strings.Join(result, "\n"), nil
}

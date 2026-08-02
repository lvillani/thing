// SPDX-License-Identifier: GPL-3.0-only

// Package skills discovers and catalogs Agent Skills conforming to the agentskills.io
// specification. It only does tier-1 progressive disclosure (discover + catalog):
// each skill's name and description are extracted so the core can advertise them
// without loading full instruction bodies.
package skills

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Skill is a discovered skill's catalog metadata and the path to its SKILL.md.
type Skill struct {
	Name        string
	Description string
	Location    string
}

// Registry holds the skills discovered from one or more skill roots.
type Registry struct {
	byName map[string]Skill
}

// New scans the given roots in order. A skill found in a later root shadows a
// same-named skill from an earlier root, so pass user-level roots before project-level
// ones. A missing root is not an error; an empty registry means no skills.
func New(roots ...string) (*Registry, error) {
	r := &Registry{byName: make(map[string]Skill)}
	for _, root := range roots {
		if err := r.scan(root); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// scan walks one skill root for directories containing a SKILL.md and loads them.
func (r *Registry) scan(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		md := filepath.Join(root, e.Name(), "SKILL.md")
		if _, err := os.Stat(md); err != nil {
			continue // not a skill (no SKILL.md)
		}
		if err := r.load(md); err != nil {
			log.Printf("skills: %s: %v", md, err)
		}
	}
	return nil
}

// load parses a skill's frontmatter and records it, warning (not failing) on
// non-fatal issues and skipping skills that cannot be cataloged.
func (r *Registry) load(md string) error {
	data, err := os.ReadFile(md)
	if err != nil {
		return err
	}
	name, desc, ok := parseFrontmatter(string(data))
	if !ok {
		return errors.New("missing or unparseable frontmatter")
	}
	if name == "" {
		return errors.New("missing name")
	}
	if desc == "" {
		return errors.New("missing description")
	}
	if dir := filepath.Base(filepath.Dir(md)); dir != name {
		log.Printf("skills: %s: name %q does not match directory %q", md, name, dir)
	}
	if strings.Contains(desc, ":") {
		log.Printf("skills: %s: description for %q may contain an unquoted colon (subtly malformed YAML); loading anyway", md, name)
	}
	r.byName[name] = Skill{Name: name, Description: desc, Location: md}
	return nil
}

// Catalog returns the discovered skills, sorted by name for determinism.
func (r *Registry) Catalog() []Skill {
	names := make([]string, 0, len(r.byName))
	for name := range r.byName {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]Skill, 0, len(names))
	for _, name := range names {
		out = append(out, r.byName[name])
	}
	return out
}

// parseFrontmatter extracts the name and description fields from a SKILL.md's YAML
// frontmatter block. It is deliberately lenient: it only needs the two scalar keys and
// tolerates unquoted values containing colons, which strict YAML would reject.
func parseFrontmatter(content string) (name, desc string, ok bool) {
	// Must start with a standalone `---` line.
	if content != "---" && !strings.HasPrefix(content, "---\n") {
		return "", "", false
	}
	rest := content[3:]
	nl := strings.IndexByte(rest, '\n')
	if nl < 0 {
		return "", "", false
	}
	rest = rest[nl+1:]

	end := closingDelim(rest)
	if end < 0 {
		return "", "", false
	}
	for _, line := range strings.Split(rest[:end], "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		key := strings.TrimSpace(line[:colon])
		val := strings.TrimSpace(line[colon+1:])
		switch key {
		case "name":
			name = stripQuotes(val)
		case "description":
			desc = stripQuotes(val)
		}
	}
	return name, desc, name != ""
}

// closingDelim returns the index of the first line exactly equal to `---`, or -1. It
// skips lines that merely start with `---` (e.g. a horizontal rule in a body) so the
// frontmatter block is not truncated there.
func closingDelim(s string) int {
	for {
		i := strings.Index(s, "\n---")
		if i < 0 {
			return -1
		}
		if i+4 == len(s) || s[i+4] == '\n' {
			return i
		}
		s = s[i+3:]
	}
}

// stripQuotes removes a single matching surrounding quote pair, leaving a value with
// stray or unbalanced quotes (e.g. an apostrophe) untouched.
func stripQuotes(v string) string {
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	return v
}

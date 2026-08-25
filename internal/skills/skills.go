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

	"go.yaml.in/yaml/v4"
)

// Skill is a discovered skill's catalog metadata and the path to its SKILL.md.
type Skill struct {
	Name        string
	Description string
	Location    string
	// DisableModelInvocation, when true, hides the skill from the catalog presented
	// to the model for model-driven invocation. The skill is still resolvable by
	// name (Get) and visible in the full Catalog, so user-explicit invocation can
	// still reach it.
	DisableModelInvocation bool
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
	name, desc, disableMD, ok := parseFrontmatter(string(data))
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
	r.byName[name] = Skill{
		Name:                   name,
		Description:            desc,
		Location:               md,
		DisableModelInvocation: disableMD,
	}
	return nil
}

// Get returns the skill with the given name, or ok=false if it is not discovered.
// It is used by manual activation (/skill:<name>), which must resolve a skill by
// name at request time even when the skill is not advertised in the system prompt.
func (r *Registry) Get(name string) (Skill, bool) {
	skill, ok := r.byName[name]
	return skill, ok
}

// Catalog returns the discovered skills, sorted by name for determinism. It includes
// every discovered skill regardless of DisableModelInvocation: it is the full set a
// user-explicit invoker can reach. Use ModelCatalog for the set shown to the model.
func (r *Registry) Catalog() []Skill {
	return r.catalog(func(Skill) bool { return true })
}

// ModelCatalog returns the skills usable for model-driven invocation — i.e. the
// catalog excluding those with DisableModelInvocation set. Skills hidden here are
// still discoverable via Catalog and Get, so user-explicit invocation keeps working.
func (r *Registry) ModelCatalog() []Skill {
	return r.catalog(func(s Skill) bool { return !s.DisableModelInvocation })
}

// catalog returns the discovered skills matching the predicate, sorted by name.
func (r *Registry) catalog(keep func(Skill) bool) []Skill {
	names := make([]string, 0, len(r.byName))
	for name, s := range r.byName {
		if keep(s) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	out := make([]Skill, 0, len(names))
	for _, name := range names {
		out = append(out, r.byName[name])
	}
	return out
}

// frontmatter is the metadata block at the start of a SKILL.md file.
type frontmatter struct {
	Name                   string `yaml:"name"`
	Description            string `yaml:"description"`
	DisableModelInvocation bool   `yaml:"disable-model-invocation"`
}

// parseFrontmatter extracts the metadata from a SKILL.md's YAML frontmatter block.
func parseFrontmatter(content string) (name, desc string, disableMD bool, ok bool) {
	if content != "---" && !strings.HasPrefix(content, "---\n") {
		return "", "", false, false
	}

	rest := content[3:]
	nl := strings.IndexByte(rest, '\n')
	if nl < 0 {
		return "", "", false, false
	}
	rest = rest[nl+1:]

	end := closingDelim(rest)
	if end < 0 {
		return "", "", false, false
	}

	var metadata frontmatter
	if err := yaml.Unmarshal([]byte(rest[:end]), &metadata); err != nil {
		return "", "", false, false
	}
	return metadata.Name, metadata.Description, metadata.DisableModelInvocation, metadata.Name != ""
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

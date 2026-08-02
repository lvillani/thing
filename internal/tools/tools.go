// SPDX-License-Identifier: GPL-3.0-only

// Package tools implements a tool registry.
package tools

import (
	"thing/internal/model"
)

type ToolFunction func(input string) (string, error)

type Tool interface {
	Describe() model.Tool
	Run(input string) (string, error)
}

type ToolRegistry struct {
	tools      []model.Tool
	toolsFuncs map[string]ToolFunction
}

func NewToolRegistry() *ToolRegistry {
	r := ToolRegistry{toolsFuncs: make(map[string]ToolFunction)}
	r.Register(&bash{})

	return &r
}

// Register adds a tool so the model can invoke it.
func (r *ToolRegistry) Register(tool Tool) {
	desc := tool.Describe()
	r.tools = append(r.tools, desc)
	r.toolsFuncs[desc.Function.Name] = tool.Run
}

func (r *ToolRegistry) Tools() []model.Tool {
	return r.tools
}

func (r *ToolRegistry) Run(name string, input string) (string, error) {
	if f, ok := r.toolsFuncs[name]; ok {
		return f(input)
	}
	return "", nil
}

// SPDX-License-Identifier: GPL-3.0-only

// Package tools implements a tool registry.
package tools

import (
	"fmt"

	"thing/internal/model"
)

// runFunction is the callable function signature for a tool.
type runFunction func(input model.ToolCallFunctionArguments) (string, error)

// Tool is the interface that all tools must implement to be registered with the tool
// registry.
type Tool interface {
	Describe() model.Tool
	Run(input model.ToolCallFunctionArguments) (string, error)
}

// Registry is a registry of tools that can be invoked by the model.
type Registry struct {
	tools        []model.Tool
	runFunctions map[string]runFunction
}

// NewRegistry returns a new tool registry with all built-in tools already registered.
func NewRegistry() *Registry {
	r := Registry{runFunctions: make(map[string]runFunction)}
	r.Register(&bash{})

	return &r
}

// Register adds a tool so the model can invoke it.
func (r *Registry) Register(tool Tool) {
	desc := tool.Describe()

	if _, ok := r.runFunctions[desc.Function.Name]; ok {
		return
	}

	r.tools = append(r.tools, desc)
	r.runFunctions[desc.Function.Name] = tool.Run
}

// Tools returns the list of tools registered with the registry.
func (r *Registry) Tools() []model.Tool {
	return r.tools
}

// Run executes the tool with the given name and input.
func (r *Registry) Run(name string, input model.ToolCallFunctionArguments) (string, error) {
	f, ok := r.runFunctions[name]
	if !ok {
		return "", fmt.Errorf("tool %q not found", name)
	}

	return f(input)
}

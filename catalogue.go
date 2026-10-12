package ahasendmcp

import (
	"context"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Toolset names.
const (
	ToolsetSending     = "sending"
	ToolsetDiagnostics = "diagnostics"
	ToolsetDomains     = "domains"
)

// toolsets lists every toolset, in catalogue order.
var toolsets = []string{ToolsetSending, ToolsetDiagnostics, ToolsetDomains}

// ToolInfo describes one tool of the catalogue.
type ToolInfo struct {
	Name, Title, Toolset string
	RequiredScopes       []string // static scopes and scope families
	Annotations          mcp.ToolAnnotations
}

// Catalogue returns every tool's ToolInfo.
func Catalogue() []ToolInfo {
	tools := allTools()
	infos := make([]ToolInfo, 0, len(tools))
	for _, t := range tools {
		info := t.info
		info.RequiredScopes = slices.Clone(t.info.RequiredScopes)
		info.Annotations.DestructiveHint = clonePtr(t.info.Annotations.DestructiveHint)
		info.Annotations.OpenWorldHint = clonePtr(t.info.Annotations.OpenWorldHint)
		infos = append(infos, info)
	}
	return infos
}

// OfferedScopes returns the offered scopes: each static required scope,
// and each scope family as "<family>:all".
func OfferedScopes() []string {
	var offered []string
	for _, t := range allTools() {
		for _, s := range t.info.RequiredScopes {
			if isScopeFamily(s) {
				s += ":all"
			}
			if !slices.Contains(offered, s) {
				offered = append(offered, s)
			}
		}
	}
	slices.Sort(offered)
	return offered
}

// allTools returns every tool, grouped by toolset in catalogue order.
func allTools() []tool {
	var tools []tool
	tools = append(tools, sendingTools()...)
	tools = append(tools, diagnosticsTools()...)
	tools = append(tools, domainsTools()...)
	return tools
}

// tool is one catalogue entry with its handler.
type tool struct {
	info ToolInfo
	// description is the text the agent reads. describe, when set, replaces
	// it with a text that depends on the connection.
	description string
	describe    func(env *toolEnv) string
	// add registers the tool and its handler on a server.
	add func(s *mcp.Server, t *mcp.Tool, env *toolEnv)
}

// mcpTool returns the MCP tool definition for this connection.
func (t tool) mcpTool(env *toolEnv) *mcp.Tool {
	description := t.description
	if t.describe != nil {
		description = t.describe(env)
	}
	annotations := t.info.Annotations
	return &mcp.Tool{
		Name:        t.info.Name,
		Title:       t.info.Title,
		Description: description,
		Annotations: &annotations,
	}
}

// handlerFunc is a tool's typed handler. An error that is not a *toolError
// is mapped by toolErrorFrom.
type handlerFunc[In, Out any] func(ctx context.Context, env *toolEnv, in In) (Out, error)

// newTool builds a catalogue entry with typed input and output, so the MCP
// SDK publishes both schemas.
func newTool[In, Out any](info ToolInfo, description string, h handlerFunc[In, Out]) tool {
	return tool{
		info:        info,
		description: description,
		add: func(s *mcp.Server, t *mcp.Tool, env *toolEnv) {
			mcp.AddTool(s, t, func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
				out, err := h(ctx, env, in)
				if err != nil {
					var zero Out
					return nil, zero, toolErrorFrom(err)
				}
				return nil, out, nil
			})
		},
	}
}

// Annotation sets, one per letter combination the catalogue uses.

// annRead is a read-only tool.
func annRead() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: ptr(false), OpenWorldHint: ptr(false)}
}

// annWrite is a tool that writes and destroys nothing.
func annWrite() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{DestructiveHint: ptr(false), OpenWorldHint: ptr(false)}
}

// annWriteIdempotent is a writing tool that can be repeated without
// further effect.
func annWriteIdempotent() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{DestructiveHint: ptr(false), IdempotentHint: true, OpenWorldHint: ptr(false)}
}

// annWriteOpenWorld is a writing tool that reaches people outside the
// account.
func annWriteOpenWorld() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{DestructiveHint: ptr(false), OpenWorldHint: ptr(true)}
}

// annDestructive is a tool that removes or ends something.
func annDestructive() mcp.ToolAnnotations {
	return mcp.ToolAnnotations{DestructiveHint: ptr(true), OpenWorldHint: ptr(false)}
}

func ptr[T any](v T) *T { return &v }

func clonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

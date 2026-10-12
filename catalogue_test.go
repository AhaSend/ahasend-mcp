package ahasendmcp

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalogueEntriesAreComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, info := range Catalogue() {
		t.Run(info.Name, func(t *testing.T) {
			assert.NotEmpty(t, info.Title)
			assert.NotEmpty(t, info.Name)
			assert.LessOrEqual(t, len(info.Name), 64)
			assert.Regexp(t, `^[a-z][a-z0-9_]*$`, info.Name)
			assert.NotNil(t, info.Annotations.DestructiveHint, "DestructiveHint")
			assert.NotNil(t, info.Annotations.OpenWorldHint, "OpenWorldHint")
			assert.NotEmpty(t, info.RequiredScopes)
			assert.Contains(t, toolsets, info.Toolset)
			assert.False(t, seen[info.Name], "duplicate tool name")
			seen[info.Name] = true
		})
	}
}

func TestCatalogueToolsHaveDescriptions(t *testing.T) {
	for _, readInbound := range []bool{false, true} {
		env := &toolEnv{readInbound: readInbound}
		for _, tl := range allTools() {
			assert.NotEmpty(t, tl.mcpTool(env).Description, tl.info.Name)
		}
	}
}

// annotations is the four hint values of a tool.
type annotations struct {
	readOnly, destructive, idempotent, openWorld bool
}

var (
	read        = annotations{readOnly: true}
	write       = annotations{}
	writeIdem   = annotations{idempotent: true}
	writeOpen   = annotations{openWorld: true}
	destructive = annotations{destructive: true}
)

func TestCatalogueScopesAndAnnotations(t *testing.T) {
	want := []struct {
		name    string
		toolset string
		scopes  []string
		ann     annotations
	}{
		{"send_email", ToolsetSending, []string{"messages:send"}, writeOpen},
		{"send_conversation_email", ToolsetSending, []string{"messages:send"}, writeOpen},
		{"send_template_email", ToolsetSending, []string{"messages:send"}, writeOpen},
		{"cancel_scheduled_email", ToolsetSending, []string{"messages:cancel"}, destructive},
		{"list_messages", ToolsetDiagnostics, []string{"messages:read"}, read},
		{"get_message", ToolsetDiagnostics, []string{"messages:read"}, read},
		{"diagnose_message", ToolsetDiagnostics, []string{"messages:read"}, read},
		{"get_deliverability_stats", ToolsetDiagnostics, []string{"statistics-transactional:read"}, read},
		{"get_bounce_stats", ToolsetDiagnostics, []string{"statistics-transactional:read"}, read},
		{"get_delivery_time_stats", ToolsetDiagnostics, []string{"statistics-transactional:read"}, read},
		{"list_domains", ToolsetDomains, []string{"domains:read"}, read},
		{"get_domain", ToolsetDomains, []string{"domains:read"}, read},
		{"add_domain", ToolsetDomains, []string{"domains:write"}, write},
		{"check_domain_dns", ToolsetDomains, []string{"domains:write"}, writeIdem},
		{"update_domain", ToolsetDomains, []string{"domains:write"}, write},
		{"delete_domain", ToolsetDomains, []string{"domains:delete"}, destructive},
	}

	catalogue := Catalogue()
	require.Len(t, catalogue, len(want), "every tool is in the table")
	for i, w := range want {
		t.Run(w.name, func(t *testing.T) {
			info := catalogue[i]
			require.Equal(t, w.name, info.Name)
			assert.Equal(t, w.toolset, info.Toolset)
			assert.Equal(t, w.scopes, info.RequiredScopes)
			got := annotations{
				readOnly:    info.Annotations.ReadOnlyHint,
				destructive: *info.Annotations.DestructiveHint,
				idempotent:  info.Annotations.IdempotentHint,
				openWorld:   *info.Annotations.OpenWorldHint,
			}
			assert.Equal(t, w.ann, got)
		})
	}
}

func TestCatalogueReturnsCopies(t *testing.T) {
	first := Catalogue()
	first[0].RequiredScopes[0] = "changed"
	*first[0].Annotations.DestructiveHint = true
	second := Catalogue()
	assert.NotEqual(t, "changed", second[0].RequiredScopes[0])
	assert.False(t, *second[0].Annotations.DestructiveHint)
}

func TestOfferedScopes(t *testing.T) {
	offered := OfferedScopes()
	assert.Equal(t, []string{
		"domains:delete:all",
		"domains:read",
		"domains:write",
		"messages:cancel:all",
		"messages:read:all",
		"messages:send:all",
		"statistics-transactional:read:all",
	}, offered)

	for _, info := range Catalogue() {
		for _, scope := range info.RequiredScopes {
			if isScopeFamily(scope) {
				assert.Contains(t, offered, scope+":all", "family %s", scope)
				assert.NotContains(t, offered, scope)
			} else {
				assert.Contains(t, offered, scope, "static %s", scope)
			}
		}
	}
	assert.True(t, slices.IsSorted(offered))
}

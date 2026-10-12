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
		{"list_templates", ToolsetTemplates, []string{"templates:read"}, read},
		{"get_template", ToolsetTemplates, []string{"templates:read"}, read},
		{"create_template", ToolsetTemplates, []string{"templates:write"}, write},
		{"update_template", ToolsetTemplates, []string{"templates:write"}, write},
		{"delete_template", ToolsetTemplates, []string{"templates:delete"}, destructive},
		{"get_template_draft", ToolsetTemplates, []string{"templates:read"}, read},
		{"discard_template_draft", ToolsetTemplates, []string{"templates:write"}, destructive},
		{"publish_template", ToolsetTemplates, []string{"templates:write"}, write},
		{"list_template_versions", ToolsetTemplates, []string{"templates:read"}, read},
		{"get_template_version", ToolsetTemplates, []string{"templates:read"}, read},
		{"restore_template_version", ToolsetTemplates, []string{"templates:write"}, write},
		{"list_campaign_templates", ToolsetTemplates, []string{"templates:read", "campaigns:read"}, read},
		{"get_campaign_template", ToolsetTemplates, []string{"templates:read", "campaigns:read"}, read},
		{"create_campaign_template", ToolsetTemplates, []string{"templates:write", "campaigns:read"}, write},
		{"update_campaign_template", ToolsetTemplates, []string{"templates:write", "campaigns:read"}, write},
		{"delete_campaign_template", ToolsetTemplates, []string{"templates:delete", "campaigns:read"}, destructive},
		{"list_contacts", ToolsetAudience, []string{"contacts:read"}, read},
		{"get_contact", ToolsetAudience, []string{"contacts:read"}, read},
		{"create_contact", ToolsetAudience, []string{"contacts:write"}, write},
		{"upsert_contacts", ToolsetAudience, []string{"contacts:write"}, write},
		{"update_contact", ToolsetAudience, []string{"contacts:write"}, write},
		{"delete_contact", ToolsetAudience, []string{"contacts:delete"}, destructive},
		{"list_contact_lists", ToolsetAudience, []string{"lists:read"}, read},
		{"list_lists", ToolsetAudience, []string{"lists:read"}, read},
		{"get_list", ToolsetAudience, []string{"lists:read"}, read},
		{"create_list", ToolsetAudience, []string{"lists:write"}, write},
		{"update_list", ToolsetAudience, []string{"lists:write"}, write},
		{"delete_list", ToolsetAudience, []string{"lists:delete"}, destructive},
		{"list_list_contacts", ToolsetAudience, []string{"lists:read"}, read},
		{"add_contacts_to_list", ToolsetAudience, []string{"lists:write"}, write},
		{"upsert_list_contact", ToolsetAudience, []string{"lists:write"}, write},
		{"remove_contact_from_list", ToolsetAudience, []string{"lists:write"}, destructive},
		{"list_campaigns", ToolsetCampaigns, []string{"campaigns:read"}, read},
		{"get_campaign", ToolsetCampaigns, []string{"campaigns:read"}, read},
		{"create_campaign", ToolsetCampaigns, []string{"campaigns:write"}, write},
		{"update_campaign", ToolsetCampaigns, []string{"campaigns:write"}, write},
		{"delete_campaign", ToolsetCampaigns, []string{"campaigns:delete"}, destructive},
		{"duplicate_campaign", ToolsetCampaigns, []string{"campaigns:write"}, write},
		{"send_campaign_test", ToolsetCampaigns, []string{"campaigns:send"}, writeOpen},
		{"send_campaign", ToolsetCampaigns, []string{"campaigns:send", "campaigns:read", "lists:read"}, writeOpen},
		{"schedule_campaign", ToolsetCampaigns, []string{"campaigns:send", "campaigns:read", "lists:read"}, writeOpen},
		{"unschedule_campaign", ToolsetCampaigns, []string{"campaigns:send"}, write},
		{"get_campaign_stats", ToolsetCampaigns, []string{"campaigns:read"}, read},
		{"list_campaign_recipients", ToolsetCampaigns, []string{"campaigns:read"}, read},
		{"get_account", ToolsetManagement, []string{"accounts:read"}, read},
		{"list_webhooks", ToolsetManagement, []string{"webhooks:read"}, read},
		{"get_webhook", ToolsetManagement, []string{"webhooks:read"}, read},
		{"create_webhook", ToolsetManagement, []string{"webhooks:write"}, write},
		{"update_webhook", ToolsetManagement, []string{"webhooks:write"}, write},
		{"delete_webhook", ToolsetManagement, []string{"webhooks:delete"}, destructive},
		{"list_routes", ToolsetManagement, []string{"routes:read"}, read},
		{"get_route", ToolsetManagement, []string{"routes:read"}, read},
		{"create_route", ToolsetManagement, []string{"routes:write"}, write},
		{"update_route", ToolsetManagement, []string{"routes:write"}, write},
		{"delete_route", ToolsetManagement, []string{"routes:delete"}, destructive},
		{"list_suppressions", ToolsetManagement, []string{"suppressions:read"}, read},
		{"add_suppression", ToolsetManagement, []string{"suppressions:write"}, write},
		{"remove_suppression", ToolsetManagement, []string{"suppressions:delete"}, destructive},
		{"list_smtp_credentials", ToolsetManagement, []string{"smtp-credentials:read"}, read},
		{"get_smtp_credential", ToolsetManagement, []string{"smtp-credentials:read"}, read},
		{"delete_smtp_credential", ToolsetManagement, []string{"smtp-credentials:delete"}, destructive},
		{"list_sub_accounts", ToolsetManagement, []string{"sub-accounts:read"}, read},
		{"get_sub_account", ToolsetManagement, []string{"sub-accounts:read"}, read},
		{"create_sub_account", ToolsetManagement, []string{"sub-accounts:write"}, write},
		{"update_sub_account", ToolsetManagement, []string{"sub-accounts:write"}, write},
		{"get_sub_account_usage", ToolsetManagement, []string{"sub-accounts:usage"}, read},
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
		"accounts:read",
		"campaigns:delete",
		"campaigns:read",
		"campaigns:send",
		"campaigns:write",
		"contacts:delete",
		"contacts:read",
		"contacts:write",
		"domains:delete:all",
		"domains:read",
		"domains:write",
		"lists:delete",
		"lists:read",
		"lists:write",
		"messages:cancel:all",
		"messages:read:all",
		"messages:send:all",
		"routes:delete:all",
		"routes:read:all",
		"routes:write:all",
		"smtp-credentials:delete:all",
		"smtp-credentials:read:all",
		"statistics-transactional:read:all",
		"sub-accounts:read",
		"sub-accounts:usage",
		"sub-accounts:write",
		"suppressions:delete",
		"suppressions:read",
		"suppressions:write",
		"templates:delete",
		"templates:read",
		"templates:write",
		"webhooks:delete:all",
		"webhooks:read:all",
		"webhooks:write:all",
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

package ahasendmcp

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateTools(t *testing.T) {
	tpl := "/templates/" + testTemplateID
	version := map[string]any{
		"object": "template_version", "id": testVersionID, "version": 3, "published_at": fixtureTime,
		"published_by": map[string]any{"type": "api_key", "id": testContactID},
		"subject":      "Hello", "preheader": "", "variables": []map[string]any{},
		"content": map[string]any{"html": "<p>VERSION-HTML</p>"},
	}
	published := templateJSON()
	published["subject"] = "Published subject"
	published["variables"] = []map[string]any{{"name": "code", "required": true}}
	published["content"] = map[string]any{"html": "<p>PUBLISHED-HTML</p>"}

	runToolCases(t, []toolCase{
		{
			tool: "list_templates", args: map[string]any{"limit": 5, "cursor": "c-1"},
			method: http.MethodGet, path: "/templates", wantQuery: map[string]string{"limit": "5", "after": "c-1"},
			resp: page(templateJSON()),
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, testTemplateID, field(t, out, "templates.0.id"))
				assert.Equal(t, "app@mail.example.com", field(t, out, "templates.0.from.email"))
				assert.Equal(t, true, field(t, out, "templates.0.has_draft"))
				assert.Equal(t, "next-page", out["next_cursor"])
			},
		},
		{
			tool: "get_template", args: map[string]any{"template_id": testTemplateID},
			method: http.MethodGet, path: tpl, resp: templateJSON(),
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, "<p>TEMPLATE-HTML-CONTENT</p>", field(t, out, "content.html"))
				assert.Equal(t, "first_name", field(t, out, "variables.0.name"))
				assert.Equal(t, "help@example.com", out["reply_to"])
			},
		},
		{
			tool: "create_template",
			args: map[string]any{
				"name": "Welcome", "subject": "Hello", "html": "<p>Hi</p>", "publish": true,
				"from": map[string]any{"email": "app@mail.example.com", "name": "App"}, "reply_to": "help@example.com",
			},
			method: http.MethodPost, path: "/templates", status: http.StatusCreated, resp: templateJSON(),
			wantBody: map[string]any{
				"name": "Welcome", "subject": "Hello", "publish": true,
				"from":     map[string]any{"email": "app@mail.example.com", "name": "App"},
				"reply_to": map[string]any{"email": "help@example.com"},
				"content":  map[string]any{"html": "<p>Hi</p>"},
			},
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, testTemplateID, out["id"])
				assert.NotContains(t, out, "content", "writes do not echo the content")
			},
		},
		{
			tool:   "update_template",
			args:   map[string]any{"template_id": testTemplateID, "name": "Welcome 2", "html": "<p>New</p>", "text_from_html": true},
			method: http.MethodPut, path: tpl, resp: templateJSON(),
			wantBody: map[string]any{"name": "Welcome 2", "content": map[string]any{"html": "<p>New</p>", "text": nil}},
		},
		{
			tool: "delete_template", args: map[string]any{"template_id": testTemplateID},
			method: http.MethodDelete, path: tpl, resp: success("Template deleted"),
			check: func(t *testing.T, out map[string]any) { assert.Equal(t, "Template deleted", out["message"]) },
		},
		{
			tool: "get_published_template", args: map[string]any{"template_id": testTemplateID},
			method: http.MethodGet, path: tpl + "/published", resp: published,
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, "Published subject", out["subject"])
				assert.Equal(t, "<p>PUBLISHED-HTML</p>", field(t, out, "content.html"))
				assert.Equal(t, "code", field(t, out, "variables.0.name"))
			},
		},
		{
			tool: "discard_template_draft", args: map[string]any{"template_id": testTemplateID},
			method: http.MethodDelete, path: tpl + "/draft", resp: templateJSON(),
			check: func(t *testing.T, out map[string]any) { assert.Equal(t, "Welcome", out["name"]) },
		},
		{
			tool: "publish_template", args: map[string]any{"template_id": testTemplateID},
			method: http.MethodPost, path: tpl + "/publish", resp: templateJSON(),
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, "first_name", field(t, out, "variables.0.name"))
			},
		},
		{
			tool: "list_template_versions", args: map[string]any{"template_id": testTemplateID},
			method: http.MethodGet, path: tpl + "/versions",
			resp: map[string]any{"object": "list", "data": []map[string]any{version}},
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, testVersionID, field(t, out, "versions.0.id"))
				assert.Equal(t, float64(3), field(t, out, "versions.0.version"))
				assert.Equal(t, "api_key", field(t, out, "versions.0.published_by.type"))
			},
		},
		{
			tool: "get_template_version", args: map[string]any{"template_id": testTemplateID, "version_id": testVersionID},
			method: http.MethodGet, path: tpl + "/versions/" + testVersionID, resp: version,
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, "<p>VERSION-HTML</p>", field(t, out, "content.html"))
			},
		},
		{
			tool: "restore_template_version", args: map[string]any{"template_id": testTemplateID, "version_id": testVersionID, "publish": true},
			method: http.MethodPost, path: tpl + "/versions/" + testVersionID + "/restore", resp: templateJSON(),
			wantBody: map[string]any{"publish": true},
		},
		{
			tool: "list_campaign_templates", args: map[string]any{},
			method: http.MethodGet, path: "/campaign-templates", wantQuery: map[string]string{"limit": "20"},
			resp: page(campaignTemplateJSON()),
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, testCampaignTemplateID, field(t, out, "campaign_templates.0.id"))
				assert.Equal(t, "advanced", field(t, out, "campaign_templates.0.editor"))
			},
		},
		{
			tool: "get_campaign_template", args: map[string]any{"campaign_template_id": testCampaignTemplateID},
			method: http.MethodGet, path: "/campaign-templates/" + testCampaignTemplateID, resp: campaignTemplateJSON(),
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, "<mjml>CAMPAIGN-TEMPLATE-MJML</mjml>", field(t, out, "content.mjml"))
			},
		},
		{
			tool:   "create_campaign_template",
			args:   map[string]any{"name": "Newsletter", "mjml": "<mjml></mjml>", "text": "Plain"},
			method: http.MethodPost, path: "/campaign-templates", status: http.StatusCreated, resp: campaignTemplateJSON(),
			wantBody: map[string]any{"name": "Newsletter", "content": map[string]any{"mjml": "<mjml></mjml>", "text": "Plain"}},
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, testCampaignTemplateID, out["id"])
				assert.NotContains(t, out, "content")
			},
		},
		{
			tool:   "update_campaign_template",
			args:   map[string]any{"campaign_template_id": testCampaignTemplateID, "name": "Newsletter 2"},
			method: http.MethodPut, path: "/campaign-templates/" + testCampaignTemplateID, resp: campaignTemplateJSON(),
			wantBody: map[string]any{"name": "Newsletter 2"},
		},
		{
			tool: "delete_campaign_template", args: map[string]any{"campaign_template_id": testCampaignTemplateID},
			method: http.MethodDelete, path: "/campaign-templates/" + testCampaignTemplateID, resp: success("Campaign template deleted"),
		},
	})
}

func TestTemplateToolsRefuseABadID(t *testing.T) {
	f := newFakeAPI(t)
	rec := &fakeRecorder{}
	opts := testOptions(t, f)
	opts.Recorder = rec
	cs := connect(t, opts)

	text := requireToolError(t, callTool(t, cs, "get_template", map[string]any{"template_id": "welcome"}))
	assert.Contains(t, text, "template_id")
	assert.Empty(t, f.calls())
	assert.Equal(t, ErrorKindInvalidInput, rec.only(t).ErrorKind)
}

func TestUpdateTemplateRefusesTextWithTextFromHTML(t *testing.T) {
	f := newFakeAPI(t)
	cs := connect(t, testOptions(t, f))
	res := callTool(t, cs, "update_template", map[string]any{"template_id": testTemplateID, "text": "x", "text_from_html": true})
	requireToolError(t, res)
	assert.Empty(t, f.calls())
}

func TestNoListToolResultHoldsTemplateContent(t *testing.T) {
	tests := []struct {
		tool string
		path string
		args map[string]any
		body any
	}{
		{"list_templates", "/templates", nil, page(templateJSON())},
		{"list_campaign_templates", "/campaign-templates", nil, page(campaignTemplateJSON())},
		{"list_campaigns", "/campaigns", nil, page(campaignJSON())},
		{"list_template_versions", "/templates/" + testTemplateID + "/versions", map[string]any{"template_id": testTemplateID},
			map[string]any{"object": "list", "data": []map[string]any{{
				"object": "template_version", "id": testVersionID, "version": 1, "published_at": fixtureTime,
				"content": map[string]any{"html": "<p>TEMPLATE-HTML-CONTENT</p>"},
			}}}},
	}
	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			f := newFakeAPI(t)
			f.handle("GET "+apiPath(tt.path), http.StatusOK, tt.body)
			cs := connect(t, testOptions(t, f))

			args := tt.args
			if args == nil {
				args = map[string]any{}
			}
			res := callTool(t, cs, tt.tool, args)
			require.False(t, res.IsError, resultText(res))
			raw, err := json.Marshal(res)
			require.NoError(t, err)
			for _, content := range []string{"TEMPLATE-HTML-CONTENT", "TEMPLATE-TEXT-CONTENT", "CAMPAIGN-TEMPLATE", "CAMPAIGN-HTML-CONTENT", "CAMPAIGN-TEXT"} {
				assert.NotContains(t, string(raw), content)
			}
		})
	}
}

func TestCampaignTemplateToolsNeedCampaignsRead(t *testing.T) {
	opts := testOptions(t, newFakeAPI(t))
	opts.Scopes = []string{"templates:read", "templates:write", "templates:delete"}
	names := listedNames(t, connect(t, opts))
	for _, name := range []string{"list_campaign_templates", "get_campaign_template", "create_campaign_template", "update_campaign_template", "delete_campaign_template"} {
		assert.NotContains(t, names, name)
	}
	assert.Contains(t, names, "list_templates")

	opts.Scopes = append(opts.Scopes, "campaigns:read")
	names = listedNames(t, connect(t, opts))
	for _, name := range []string{"list_campaign_templates", "get_campaign_template", "create_campaign_template", "update_campaign_template", "delete_campaign_template"} {
		assert.Contains(t, names, name)
	}
}

func TestTemplateVariablesShowTheirKind(t *testing.T) {
	body := templateJSON()
	body["variables"] = []map[string]any{
		{"name": "first_name", "required": true, "kind": "value"},
		{"name": "vip", "required": false, "kind": "condition"},
		{"name": "items", "required": true, "kind": "list", "item_fields": []string{"name", "price"}},
		{"name": "tags", "required": false, "kind": "list", "item_fields": []string{"value"}},
		{"name": "rows", "required": false, "kind": "list"},
		{"name": "legacy", "required": false},
		{"name": "future", "required": false, "kind": "matrix", "item_fields": []string{"x"}},
	}
	f := newFakeAPI(t)
	f.handle("GET "+apiPath("/templates/"+testTemplateID), http.StatusOK, body)
	cs := connect(t, testOptions(t, f))

	out := decode[map[string]any](t, callTool(t, cs, "get_template", map[string]any{"template_id": testTemplateID}))

	vars := map[string]map[string]any{}
	for _, v := range field(t, out, "variables").([]any) {
		m := v.(map[string]any)
		vars[m["name"].(string)] = m
	}
	tests := []struct {
		name           string
		wantKind       string
		wantItemFields any
	}{
		{"first_name", "value", nil},
		{"vip", "condition", nil},
		{"items", "list", []any{"name", "price"}},
		{"tags", "list", []any{"value"}},
		{"rows", "list", []any{}},
		{"legacy", "value", nil},
		{"future", "matrix", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := vars[tt.name]
			require.NotNil(t, v)
			assert.Equal(t, tt.wantKind, v["kind"])
			if tt.wantItemFields == nil {
				assert.NotContains(t, v, "item_fields")
			} else {
				assert.Equal(t, tt.wantItemFields, v["item_fields"])
			}
		})
	}
}

func TestTemplateToolsSayWhatEachVariableKindTakes(t *testing.T) {
	cs := connect(t, testOptions(t, newFakeAPI(t)))
	res, err := cs.ListTools(context.Background(), nil)
	require.NoError(t, err)
	descriptions := map[string]string{}
	var sendSchema []byte
	for _, tl := range res.Tools {
		descriptions[tl.Name] = tl.Description
		if tl.Name == "send_template_email" {
			sendSchema, err = json.Marshal(tl.InputSchema)
			require.NoError(t, err)
		}
	}
	for _, name := range []string{"get_template", "get_published_template", "get_template_version", "send_template_email"} {
		assert.Contains(t, descriptions[name], "kind", name)
		assert.Contains(t, descriptions[name], "item_fields", name)
	}
	assert.Contains(t, string(sendSchema), "each variable's kind tells what to send", "the substitutions description")
}

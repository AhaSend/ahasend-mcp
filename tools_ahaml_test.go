package ahasendmcp

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testMarkup  = `<Email version="1"><Paragraph>Hi <Var name="first_name"/></Paragraph></Email>`
	testMarkup2 = `<Email version="1"><Heading level="1">New</Heading></Email>`
)

// simpleContentJSON is the content of a Simple Editor design as the API
// returns it.
func simpleContentJSON() map[string]any {
	return map[string]any{"html": "<p>Hi {{ first_name }}</p>", "markup": testMarkup, "text": "Hi", "text_is_custom": false}
}

// markupWarningsJSON is the markup_warnings of a write that carried markup.
func markupWarningsJSON() []map[string]any {
	return []map[string]any{{
		"code": "jinja_in_text", "message": "Text holds {{, which stays text",
		"line": 1, "column": 32, "path": "Email > Paragraph[1]",
	}}
}

func simpleTemplateJSON() map[string]any {
	t := templateJSON()
	t["editor"] = "simple"
	t["content"] = simpleContentJSON()
	return t
}

func simpleCampaignTemplateJSON() map[string]any {
	t := campaignTemplateJSON()
	t["editor"] = "simple"
	t["content"] = simpleContentJSON()
	return t
}

func simpleCampaignJSON() map[string]any {
	c := campaignJSON()
	c["editor"] = "simple"
	c["content"] = simpleContentJSON()
	return c
}

// withWarnings returns a write answer with markup_warnings.
func withWarnings(body map[string]any) map[string]any {
	body["markup_warnings"] = markupWarningsJSON()
	return body
}

func TestContentToolsReturnMarkup(t *testing.T) {
	tpl := "/templates/" + testTemplateID
	version := map[string]any{
		"object": "template_version", "id": testVersionID, "version": 1, "published_at": fixtureTime,
		"variables": []map[string]any{}, "content": simpleContentJSON(),
	}
	hasMarkup := func(t *testing.T, out map[string]any) {
		assert.Equal(t, testMarkup, field(t, out, "content.markup"))
		assert.NotContains(t, out["content"], "markup_error")
	}

	runToolCases(t, []toolCase{
		{tool: "get_template", args: map[string]any{"template_id": testTemplateID},
			method: http.MethodGet, path: tpl, resp: simpleTemplateJSON(), check: hasMarkup},
		{tool: "get_published_template", args: map[string]any{"template_id": testTemplateID},
			method: http.MethodGet, path: tpl + "/published", resp: simpleTemplateJSON(), check: hasMarkup},
		{tool: "get_template_version", args: map[string]any{"template_id": testTemplateID, "version_id": testVersionID},
			method: http.MethodGet, path: tpl + "/versions/" + testVersionID, resp: version, check: hasMarkup},
		{tool: "get_campaign_template", args: map[string]any{"campaign_template_id": testCampaignTemplateID},
			method: http.MethodGet, path: "/campaign-templates/" + testCampaignTemplateID, resp: simpleCampaignTemplateJSON(), check: hasMarkup},
		{tool: "get_campaign", args: map[string]any{"campaign_id": testCampaignID},
			method: http.MethodGet, path: "/campaigns/" + testCampaignID, resp: simpleCampaignJSON(), check: hasMarkup},
	})
}

func TestContentToolsReturnMarkupError(t *testing.T) {
	tpl := simpleTemplateJSON()
	tpl["content"] = map[string]any{
		"html": "<p>Hi</p>", "text": "Hi", "text_is_custom": false,
		"markup_error": map[string]any{"code": "html_block", "message": "The design holds an HTML block"},
	}
	runToolCases(t, []toolCase{{
		tool: "get_template", args: map[string]any{"template_id": testTemplateID},
		method: http.MethodGet, path: "/templates/" + testTemplateID, resp: tpl,
		check: func(t *testing.T, out map[string]any) {
			assert.NotContains(t, out["content"], "markup")
			assert.Equal(t, "html_block", field(t, out, "content.markup_error.code"))
			assert.Equal(t, "The design holds an HTML block", field(t, out, "content.markup_error.message"))
		},
	}})
}

func TestWriteToolsSendMarkupAndReturnItsWarnings(t *testing.T) {
	hasWarnings := func(t *testing.T, out map[string]any) {
		assert.Equal(t, "jinja_in_text", field(t, out, "markup_warnings.0.code"))
		assert.Equal(t, "Text holds {{, which stays text", field(t, out, "markup_warnings.0.message"))
		assert.Equal(t, float64(1), field(t, out, "markup_warnings.0.line"))
		assert.Equal(t, float64(32), field(t, out, "markup_warnings.0.column"))
		assert.Equal(t, "Email > Paragraph[1]", field(t, out, "markup_warnings.0.path"))
	}

	runToolCases(t, []toolCase{
		{
			tool:   "create_template",
			args:   map[string]any{"name": "Welcome", "markup": testMarkup},
			method: http.MethodPost, path: "/templates", status: http.StatusCreated, resp: withWarnings(templateJSON()),
			wantBody: map[string]any{"name": "Welcome", "content": map[string]any{"markup": testMarkup}},
			check:    hasWarnings,
		},
		{
			tool:   "update_template",
			args:   map[string]any{"template_id": testTemplateID, "markup": testMarkup2, "text": "New"},
			method: http.MethodPut, path: "/templates/" + testTemplateID, resp: withWarnings(simpleTemplateJSON()),
			wantBody: map[string]any{"content": map[string]any{"markup": testMarkup2, "text": "New"}},
			check:    hasWarnings,
		},
		{
			tool:   "create_campaign_template",
			args:   map[string]any{"name": "Newsletter", "markup": testMarkup},
			method: http.MethodPost, path: "/campaign-templates", status: http.StatusCreated, resp: withWarnings(simpleCampaignTemplateJSON()),
			wantBody: map[string]any{"name": "Newsletter", "content": map[string]any{"markup": testMarkup}},
			check:    hasWarnings,
		},
		{
			tool:   "update_campaign_template",
			args:   map[string]any{"campaign_template_id": testCampaignTemplateID, "markup": testMarkup2},
			method: http.MethodPut, path: "/campaign-templates/" + testCampaignTemplateID, resp: withWarnings(simpleCampaignTemplateJSON()),
			wantBody: map[string]any{"content": map[string]any{"markup": testMarkup2}},
			check:    hasWarnings,
		},
		{
			tool:   "create_campaign",
			args:   map[string]any{"name": "October news", "markup": testMarkup},
			method: http.MethodPost, path: "/campaigns", status: http.StatusCreated, resp: withWarnings(simpleCampaignJSON()),
			wantBody: map[string]any{"name": "October news", "content": map[string]any{"markup": testMarkup}},
			check:    hasWarnings,
		},
		{
			tool:   "update_campaign",
			args:   map[string]any{"campaign_id": testCampaignID, "markup": testMarkup2, "text_from_html": true},
			method: http.MethodPut, path: "/campaigns/" + testCampaignID, resp: withWarnings(simpleCampaignJSON()),
			wantBody: map[string]any{"content": map[string]any{"markup": testMarkup2, "text": nil}},
			check:    hasWarnings,
		},
	})
}

// A null markup is the same as no markup, and an empty one is sent, so the
// API refuses it as it refuses any text that is not AhaML.
func TestNullMarkupIsNoMarkupAndEmptyMarkupIsSent(t *testing.T) {
	runToolCases(t, []toolCase{
		{
			tool:   "update_campaign_template",
			args:   map[string]any{"campaign_template_id": testCampaignTemplateID, "name": "Newsletter 2", "markup": nil},
			method: http.MethodPut, path: "/campaign-templates/" + testCampaignTemplateID, resp: simpleCampaignTemplateJSON(),
			wantBody: map[string]any{"name": "Newsletter 2"},
		},
		{
			tool:   "create_template",
			args:   map[string]any{"name": "Welcome", "editor": "simple", "markup": nil},
			method: http.MethodPost, path: "/templates", status: http.StatusCreated, resp: simpleTemplateJSON(),
			wantBody: map[string]any{"name": "Welcome", "editor": "simple"},
		},
		{
			tool:   "update_campaign",
			args:   map[string]any{"campaign_id": testCampaignID, "markup": ""},
			method: http.MethodPut, path: "/campaigns/" + testCampaignID, resp: simpleCampaignJSON(),
			wantBody: map[string]any{"content": map[string]any{"markup": ""}},
		},
		{
			tool:   "create_campaign_template",
			args:   map[string]any{"name": "Newsletter", "markup": ""},
			method: http.MethodPost, path: "/campaign-templates", status: http.StatusCreated, resp: simpleCampaignTemplateJSON(),
			wantBody: map[string]any{"name": "Newsletter", "content": map[string]any{"markup": ""}},
		},
	})
}

// A content read and sent back as it is reaches the API unchanged, with the
// markup and the HTML beside it.
func TestReadMarkupGoesBackUnchanged(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("GET "+apiPath("/campaign-templates/"+testCampaignTemplateID), http.StatusOK, simpleCampaignTemplateJSON())
	f.handle("PUT "+apiPath("/campaign-templates/"+testCampaignTemplateID), http.StatusOK, simpleCampaignTemplateJSON())
	cs := connect(t, testOptions(t, f))

	read := decode[map[string]any](t, callTool(t, cs, "get_campaign_template", map[string]any{"campaign_template_id": testCampaignTemplateID}))
	content := read["content"].(map[string]any)
	out := decode[map[string]any](t, callTool(t, cs, "update_campaign_template", map[string]any{
		"campaign_template_id": testCampaignTemplateID, "markup": content["markup"], "html": content["html"],
	}))
	assert.NotContains(t, out, "markup_warnings", "a write answer without markup_warnings has none")

	calls := f.calls()
	require.Len(t, calls, 2)
	assert.Equal(t, map[string]any{"content": map[string]any{"markup": testMarkup, "html": "<p>Hi {{ first_name }}</p>"}}, calls[1].Body)
}

func TestRefusedMarkupReachesTheAgentWithEveryProblem(t *testing.T) {
	refusal := map[string]any{
		"message": "content.markup has 2 problems",
		"errors": []map[string]any{
			{
				"code": "unknown_attribute", "message": `Paragraph has no attribute "color"`,
				"line": 4, "column": 14, "path": "Email > Section[2] > Paragraph[1]",
				"hint": `wrap the text in <Color value="..."> instead`,
			},
			{"code": "rendered_too_large", "message": "The rendered HTML is larger than 200 KB."},
		},
	}
	tests := []struct {
		tool   string
		method string
		path   string
		args   map[string]any
	}{
		{"create_template", http.MethodPost, "/templates", map[string]any{"name": "Welcome", "markup": testMarkup}},
		{"update_template", http.MethodPut, "/templates/" + testTemplateID, map[string]any{"template_id": testTemplateID, "markup": testMarkup}},
		{"create_campaign_template", http.MethodPost, "/campaign-templates", map[string]any{"name": "Newsletter", "markup": testMarkup}},
		{"update_campaign_template", http.MethodPut, "/campaign-templates/" + testCampaignTemplateID,
			map[string]any{"campaign_template_id": testCampaignTemplateID, "markup": testMarkup}},
		{"create_campaign", http.MethodPost, "/campaigns", map[string]any{"name": "October news", "markup": testMarkup}},
		{"update_campaign", http.MethodPut, "/campaigns/" + testCampaignID, map[string]any{"campaign_id": testCampaignID, "markup": testMarkup}},
	}
	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			f := newFakeAPI(t)
			f.handle(tt.method+" "+apiPath(tt.path), http.StatusBadRequest, refusal)
			rec := &fakeRecorder{}
			opts := testOptions(t, f)
			opts.Recorder = rec
			cs := connect(t, opts)

			text := requireToolError(t, callTool(t, cs, tt.tool, tt.args))
			assert.Equal(t, "AhaSend refused the request (HTTP 400): content.markup has 2 problems. "+
				"Fix these problems in markup, then write it again:\n"+
				`- line 4, column 14, Email > Section[2] > Paragraph[1]: Paragraph has no attribute "color" [unknown_attribute]. `+
				`Hint: wrap the text in <Color value="..."> instead.`+"\n"+
				"- The rendered HTML is larger than 200 KB [rendered_too_large].", text)
			assert.Equal(t, ErrorKindAPI4xx, rec.only(t).ErrorKind)
		})
	}
}

func TestA400WithoutProblemsKeepsTheAPIMessage(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("PUT "+apiPath("/templates/"+testTemplateID), http.StatusBadRequest, map[string]any{"message": "mjml is refused for a simple template"})
	cs := connect(t, testOptions(t, f))

	text := requireToolError(t, callTool(t, cs, "update_template", map[string]any{"template_id": testTemplateID, "mjml": "<mjml></mjml>"}))
	assert.Equal(t, "AhaSend refused the request (HTTP 400): mjml is refused for a simple template", text)
}

// contentTools are the tools whose content takes or returns markup.
var contentTools = []string{
	"get_template", "get_published_template", "create_template", "update_template", "get_template_version",
	"get_campaign_template", "create_campaign_template", "update_campaign_template",
	"get_campaign", "create_campaign", "update_campaign",
}

func TestContentToolDescriptionsHoldTheAhaMLSummary(t *testing.T) {
	cs := connect(t, testOptions(t, newFakeAPI(t)))
	res, err := cs.ListTools(context.Background(), nil)
	require.NoError(t, err)
	descriptions := map[string]string{}
	for _, tl := range res.Tools {
		descriptions[tl.Name] = tl.Description
	}
	for _, name := range contentTools {
		require.Contains(t, descriptions, name)
		assert.Contains(t, descriptions[name], ahamlSummary, name)
		assert.Contains(t, descriptions[name], "https://ahasend.com/docs/send-api/ahaml", name)
		assert.Contains(t, descriptions[name], ahamlReferenceURI, name)
	}
}

func TestOnlyUpdateToolsSayAnOpenEditorCanSaveOverTheWrite(t *testing.T) {
	const openEditor = "A dashboard editor that was open before this write can save over it."
	cs := connect(t, testOptions(t, newFakeAPI(t)))
	res, err := cs.ListTools(context.Background(), nil)
	require.NoError(t, err)
	var saying []string
	for _, tl := range res.Tools {
		if strings.Contains(tl.Description, openEditor) {
			saying = append(saying, tl.Name)
		}
	}
	assert.ElementsMatch(t, []string{"update_template", "update_campaign_template", "update_campaign"}, saying)
}

func TestNoTemplateOrCampaignToolSaysSimpleDesignsAreReadOnly(t *testing.T) {
	cs := connect(t, testOptions(t, newFakeAPI(t)))
	res, err := cs.ListTools(context.Background(), nil)
	require.NoError(t, err)
	checked := 0
	for _, tl := range res.Tools {
		info := catalogueEntry(t, tl.Name)
		if info.Toolset != ToolsetTemplates && info.Toolset != ToolsetCampaigns {
			continue
		}
		checked++
		raw, err := json.Marshal(tl)
		require.NoError(t, err)
		text := strings.ToLower(string(raw))
		for _, phrase := range []string{"read-only here", "changed only in the dashboard", "finish in the dashboard", "html and mjml are refused"} {
			assert.NotContains(t, text, phrase, tl.Name)
		}
	}
	assert.Equal(t, len(toolNames(ToolsetTemplates))+len(toolNames(ToolsetCampaigns)), checked)
}

// catalogueEntry returns the catalogue's ToolInfo of a tool.
func catalogueEntry(t *testing.T, name string) ToolInfo {
	t.Helper()
	for _, info := range Catalogue() {
		if info.Name == name {
			return info
		}
	}
	t.Fatalf("no tool %s in the catalogue", name)
	return ToolInfo{}
}

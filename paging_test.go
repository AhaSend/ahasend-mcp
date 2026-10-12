package ahasendmcp

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// listToolCase is a paged list tool, the path it reads and the arguments it
// needs besides paging.
type listToolCase struct {
	path string
	args map[string]any
}

// listTools are the paged list tools.
var listTools = map[string]listToolCase{
	"list_messages":            {path: "/messages"},
	"list_domains":             {path: "/domains"},
	"list_templates":           {path: "/templates"},
	"list_campaign_templates":  {path: "/campaign-templates"},
	"list_contacts":            {path: "/contacts"},
	"list_contact_lists":       {path: "/contacts/ann@example.org/lists", args: map[string]any{"contact": "ann@example.org"}},
	"list_lists":               {path: "/lists"},
	"list_list_contacts":       {path: "/lists/" + testListID + "/contacts", args: map[string]any{"list_id": testListID}},
	"list_campaigns":           {path: "/campaigns"},
	"list_campaign_recipients": {path: "/campaigns/" + testCampaignID + "/recipients", args: map[string]any{"campaign_id": testCampaignID, "status": "Delivered"}},
	"list_webhooks":            {path: "/webhooks"},
	"list_routes":              {path: "/routes"},
	"list_suppressions":        {path: "/suppressions"},
	"list_smtp_credentials":    {path: "/smtp-credentials"},
	"list_sub_accounts":        {path: "/sub-accounts"},
}

// unpagedListTools are list tools whose endpoint returns every item.
var unpagedListTools = []string{"list_template_versions"}

func (c listToolCase) withArgs(extra map[string]any) map[string]any {
	args := map[string]any{}
	maps.Copy(args, c.args)
	maps.Copy(args, extra)
	return args
}

func TestEveryListToolIsCovered(t *testing.T) {
	for _, name := range toolNames("") {
		if strings.HasPrefix(name, "list_") {
			_, paged := listTools[name]
			assert.True(t, paged || slices.Contains(unpagedListTools, name), name)
		}
	}
}

func TestUnpagedListToolsTakeNoLimitOrCursor(t *testing.T) {
	cs := connect(t, testOptions(t, newFakeAPI(t)))
	res, err := cs.ListTools(context.Background(), nil)
	require.NoError(t, err)
	for _, tl := range res.Tools {
		if !slices.Contains(unpagedListTools, tl.Name) {
			continue
		}
		raw, err := json.Marshal(tl.InputSchema)
		require.NoError(t, err)
		var schema struct {
			Properties map[string]any `json:"properties"`
		}
		require.NoError(t, json.Unmarshal(raw, &schema))
		assert.NotContains(t, schema.Properties, "limit", tl.Name)
		assert.NotContains(t, schema.Properties, "cursor", tl.Name)
	}
}

func TestListToolsSendDefaultLimit(t *testing.T) {
	for name, tc := range listTools {
		t.Run(name, func(t *testing.T) {
			f := newFakeAPI(t)
			f.handle("GET "+apiPath(tc.path), http.StatusOK, messagePage(""))
			cs := connect(t, testOptions(t, f))

			res := callTool(t, cs, name, tc.withArgs(nil))
			assert.False(t, res.IsError, resultText(res))
			req := f.onlyCall(t)
			assert.Equal(t, "20", req.Query.Get("limit"))
			assert.Empty(t, req.Query.Get("after"))
		})
	}
}

func TestListToolsRefuseALimitOver100(t *testing.T) {
	for name, tc := range listTools {
		t.Run(name, func(t *testing.T) {
			f := newFakeAPI(t)
			rec := &fakeRecorder{}
			opts := testOptions(t, f)
			opts.Recorder = rec
			cs := connect(t, opts)

			text := requireToolError(t, callTool(t, cs, name, tc.withArgs(map[string]any{"limit": 101})))
			assert.Contains(t, text, "limit")
			assert.Empty(t, f.calls())
			assert.Equal(t, ErrorKindInvalidInput, rec.only(t).ErrorKind)
		})
	}
}

func TestListToolsAcceptALimitOf100(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("GET "+apiPath("/domains"), http.StatusOK, messagePage(""))
	cs := connect(t, testOptions(t, f))
	res := callTool(t, cs, "list_domains", map[string]any{"limit": 100})
	assert.False(t, res.IsError, resultText(res))
	assert.Equal(t, "100", f.onlyCall(t).Query.Get("limit"))
}

func TestListMessagesHoldsNoMessageBody(t *testing.T) {
	f := newFakeAPI(t)
	msg := messageJSON(outboundMessageID, "outbound")
	msg["content"] = "RAW-BODY-TEXT"
	msg["content_parsed"] = map[string]any{
		"parts": []map[string]any{{"content_type": "text/html", "content": "<p>PARSED-BODY-TEXT</p>"}},
	}
	f.handle("GET "+apiPath("/messages"), http.StatusOK, messagePage("", msg))
	opts := testOptions(t, f)
	opts.ReadInbound = true
	cs := connect(t, opts)

	res := callTool(t, cs, "list_messages", map[string]any{})
	require.False(t, res.IsError)
	raw, err := json.Marshal(res)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "RAW-BODY-TEXT")
	assert.NotContains(t, string(raw), "PARSED-BODY-TEXT")
}

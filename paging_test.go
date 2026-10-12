package ahasendmcp

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// listTools are the list tools and the path each reads.
var listTools = map[string]string{
	"list_messages": "/messages",
	"list_domains":  "/domains",
}

func TestEveryListToolIsCovered(t *testing.T) {
	for _, name := range toolNames("") {
		if len(name) > 5 && name[:5] == "list_" {
			assert.Contains(t, listTools, name)
		}
	}
}

func TestListToolsSendDefaultLimit(t *testing.T) {
	for name, path := range listTools {
		t.Run(name, func(t *testing.T) {
			f := newFakeAPI(t)
			f.handle("GET "+apiPath(path), http.StatusOK, messagePage(""))
			cs := connect(t, testOptions(t, f))

			res := callTool(t, cs, name, map[string]any{})
			assert.False(t, res.IsError, resultText(res))
			req := f.onlyCall(t)
			assert.Equal(t, "20", req.Query.Get("limit"))
			assert.Empty(t, req.Query.Get("after"))
		})
	}
}

func TestListToolsRefuseALimitOver100(t *testing.T) {
	for name := range listTools {
		t.Run(name, func(t *testing.T) {
			f := newFakeAPI(t)
			rec := &fakeRecorder{}
			opts := testOptions(t, f)
			opts.Recorder = rec
			cs := connect(t, opts)

			text := requireToolError(t, callTool(t, cs, name, map[string]any{"limit": 101}))
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

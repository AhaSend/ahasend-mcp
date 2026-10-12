package ahasendmcp

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIErrorsMapToToolErrors(t *testing.T) {
	const apiMessage = "the API explains the problem"
	tests := []struct {
		status   int
		contains []string
		excludes string
		kind     string
	}{
		{http.StatusBadRequest, []string{apiMessage, "400"}, "", ErrorKindAPI4xx},
		{http.StatusUnauthorized, []string{"credentials", "paused", "deleted", "dashboard"}, apiMessage, ErrorKindAPI4xx},
		{http.StatusForbidden, []string{apiMessage, "API Keys page", "connecting again"}, "", ErrorKindAPI4xx},
		{http.StatusNotFound, []string{"Not found in this AhaSend account: " + apiMessage + "."}, "", ErrorKindAPI4xx},
		{http.StatusConflict, []string{apiMessage, "409"}, "", ErrorKindAPI4xx},
		{http.StatusUnprocessableEntity, []string{apiMessage, "422"}, "", ErrorKindAPI4xx},
		{http.StatusTooManyRequests, []string{"Wait", "retry", apiMessage}, "next day", ErrorKindAPI4xx},
		{http.StatusInternalServerError, []string{"Retry later"}, apiMessage, ErrorKindAPI5xx},
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			f := newFakeAPI(t)
			f.handle("GET "+apiPath("/domains/{domain}"), tt.status, map[string]any{"message": apiMessage})
			rec := &fakeRecorder{}
			opts := testOptions(t, f)
			opts.Recorder = rec
			cs := connect(t, opts)

			text := requireToolError(t, callTool(t, cs, "get_domain", map[string]any{"domain": "mail.example.com"}))
			for _, want := range tt.contains {
				assert.Contains(t, text, want)
			}
			if tt.excludes != "" {
				assert.NotContains(t, text, tt.excludes)
			}
			call := rec.only(t)
			assert.False(t, call.Success)
			assert.Equal(t, tt.kind, call.ErrorKind)
		})
	}
}

func TestNetworkFailureMapsToRetryLater(t *testing.T) {
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()

	rec := &fakeRecorder{}
	opts := testOptions(t, newFakeAPI(t))
	opts.Client = testClient(t, closed.URL)
	opts.Recorder = rec
	cs := connect(t, opts)

	text := requireToolError(t, callTool(t, cs, "get_domain", map[string]any{"domain": "mail.example.com"}))
	assert.Contains(t, text, "Retry later")
	assert.Equal(t, ErrorKindAPI5xx, rec.only(t).ErrorKind)
}

func TestRecorderGetsSuccessAndDuration(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("GET "+apiPath("/domains/{domain}"), http.StatusOK, domainJSON("mail.example.com"))
	rec := &fakeRecorder{}
	opts := testOptions(t, f)
	opts.Recorder = rec
	cs := connect(t, opts)

	res := callTool(t, cs, "get_domain", map[string]any{"domain": "mail.example.com"})
	require.False(t, res.IsError)

	call := rec.only(t)
	assert.Equal(t, "get_domain", call.Tool)
	assert.Equal(t, ToolsetDomains, call.Toolset)
	assert.True(t, call.Success)
	assert.Empty(t, call.ErrorKind)
	assert.Greater(t, call.Duration, time.Duration(0))
}

func TestRecorderGetsInvalidInputForArgumentsTheSchemaRefuses(t *testing.T) {
	f := newFakeAPI(t)
	rec := &fakeRecorder{}
	opts := testOptions(t, f)
	opts.Recorder = rec
	cs := connect(t, opts)

	requireToolError(t, callTool(t, cs, "get_domain", map[string]any{"domain": 42}))
	assert.Empty(t, f.calls())
	assert.Equal(t, ErrorKindInvalidInput, rec.only(t).ErrorKind)
}

func TestRecorderSkipsListingAndUnknownTools(t *testing.T) {
	rec := &fakeRecorder{}
	opts := testOptions(t, newFakeAPI(t))
	opts.Scopes = []string{"domains:read"}
	opts.Recorder = rec
	cs := connect(t, opts)

	listedNames(t, cs)
	_, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: "send_email", Arguments: map[string]any{}})
	require.Error(t, err)
	assert.Empty(t, rec.calls)
}

func TestDailyLimitSaysItLiftsTheNextDay(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("GET "+apiPath("/domains/{domain}"), http.StatusTooManyRequests, map[string]any{"message": "daily test send limit reached"})
	cs := connect(t, testOptions(t, f))

	text := requireToolError(t, callTool(t, cs, "get_domain", map[string]any{"domain": "mail.example.com"}))
	assert.Contains(t, text, "daily test send limit reached")
	assert.Contains(t, text, "next day")
}

func TestMarketingToolNotFoundNamesTheMarketingCause(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("GET "+apiPath("/lists"), http.StatusNotFound, map[string]any{"message": "Not Found"})
	f.handle("GET "+apiPath("/domains/{domain}"), http.StatusNotFound, map[string]any{"message": "Not Found"})
	cs := connect(t, testOptions(t, f))

	assert.Contains(t, requireToolError(t, callTool(t, cs, "list_lists", map[string]any{})), "marketing turned on")
	assert.NotContains(t, requireToolError(t, callTool(t, cs, "get_domain", map[string]any{"domain": "mail.example.com"})), "marketing")
}

func TestNotFoundKeepsTheAPIMessage(t *testing.T) {
	tests := []struct {
		apiMessage string
		want       string
	}{
		{"list not found", "Not found in this AhaSend account: list not found."},
		{"Not Found", "Not found in this AhaSend account."},
		{"", "Not found in this AhaSend account."},
	}
	for _, tt := range tests {
		t.Run(tt.apiMessage, func(t *testing.T) {
			f := newFakeAPI(t)
			f.handle("GET "+apiPath("/domains/{domain}"), http.StatusNotFound, map[string]any{"message": tt.apiMessage})
			cs := connect(t, testOptions(t, f))
			assert.Equal(t, tt.want, requireToolError(t, callTool(t, cs, "get_domain", map[string]any{"domain": "mail.example.com"})))
		})
	}
}

func TestMarketingNoteOnlyForABareNotFound(t *testing.T) {
	tests := []struct {
		apiMessage string
		wantNote   bool
	}{
		{"", true},
		{"Not Found", true},
		{"campaign not found", false},
	}
	for _, tt := range tests {
		t.Run(tt.apiMessage, func(t *testing.T) {
			f := newFakeAPI(t)
			f.handle("GET "+apiPath("/campaigns/"+testCampaignID), http.StatusNotFound, map[string]any{"message": tt.apiMessage})
			cs := connect(t, testOptions(t, f))

			text := requireToolError(t, callTool(t, cs, "get_campaign", map[string]any{"campaign_id": testCampaignID}))
			if tt.wantNote {
				assert.Equal(t, "Not found in this AhaSend account. Contacts, lists, campaigns and campaign "+
					"templates exist only on accounts with marketing turned on.", text)
				return
			}
			assert.Equal(t, "Not found in this AhaSend account: campaign not found.", text)
		})
	}
}

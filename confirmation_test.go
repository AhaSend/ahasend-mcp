package ahasendmcp

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	protocol20260728 = "2026-07-28"
	protocol20251125 = "2025-11-25"
	protocol20250618 = "2025-06-18"
)

// campaignFake is a fake API holding one draft campaign on two lists, with
// two ready segments it can use, whose fields a test can change between
// calls.
type campaignFake struct {
	*fakeAPI
	mu       sync.Mutex
	campaign map[string]any
	segments map[string]map[string]any
}

func newCampaignFake(t *testing.T) *campaignFake {
	t.Helper()
	cf := &campaignFake{fakeAPI: newFakeAPI(t), campaign: campaignJSON(), segments: map[string]map[string]any{
		testSegmentID:  segmentJSON(testSegmentID, "Engaged", "ready", 380),
		testSegment2ID: segmentJSON(testSegment2ID, "Lapsed", "ready", 50),
	}}
	cf.handleFunc("GET "+apiPath("/campaigns/"+testCampaignID), func(*http.Request) (int, any) {
		cf.mu.Lock()
		defer cf.mu.Unlock()
		return http.StatusOK, cf.campaign
	})
	cf.handle("GET "+apiPath("/lists/"+testListID), http.StatusOK, listJSON(testListID, "Newsletter", 1200))
	cf.handle("GET "+apiPath("/lists/"+testList2ID), http.StatusOK, listJSON(testList2ID, "Customers", 345))
	cf.handleFunc("GET "+apiPath("/segments/{id}"), func(r *http.Request) (int, any) {
		cf.mu.Lock()
		defer cf.mu.Unlock()
		return http.StatusOK, cf.segments[r.PathValue("id")]
	})
	cf.handleFunc("POST "+apiPath("/campaigns/"+testCampaignID+"/send"), func(*http.Request) (int, any) {
		cf.mu.Lock()
		sent := maps.Clone(cf.campaign)
		cf.mu.Unlock()
		sent["status"] = "validating"
		sent["warnings"] = []string{}
		return http.StatusOK, sent
	})
	return cf
}

// set changes one field of the campaign.
func (cf *campaignFake) set(key string, value any) {
	cf.mu.Lock()
	defer cf.mu.Unlock()
	next := maps.Clone(cf.campaign)
	next[key] = value
	cf.campaign = next
}

// setSegment replaces one segment.
func (cf *campaignFake) setSegment(id string, segment map[string]any) {
	cf.mu.Lock()
	defer cf.mu.Unlock()
	cf.segments[id] = segment
}

// sends returns the send requests the fake received.
func (cf *campaignFake) sends() []recordedRequest {
	var out []recordedRequest
	for _, c := range cf.calls() {
		if c.Method == http.MethodPost && c.Path == apiPath("/campaigns/"+testCampaignID+"/send") {
			out = append(out, c)
		}
	}
	return out
}

func (r *fakeRecorder) all() []ToolCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]ToolCall(nil), r.calls...)
}

// manualClient answers no input request itself, so a test drives each round.
func manualClient(t *testing.T) *mcp.ClientOptions {
	return &mcp.ClientOptions{
		MultiRoundTrip: &mcp.MultiRoundTripOptions{Disabled: true},
		ElicitationHandler: func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			t.Error("the client must not be asked directly")
			return &mcp.ElicitResult{Action: "cancel"}, nil
		},
	}
}

// answeringClient answers every question with result and keeps the
// questions it was asked.
type answeringClient struct {
	mu     sync.Mutex
	asked  []*mcp.ElicitParams
	result *mcp.ElicitResult
	before func()
}

func (a *answeringClient) options() *mcp.ClientOptions {
	return &mcp.ClientOptions{ElicitationHandler: a.handle}
}

func (a *answeringClient) handle(_ context.Context, req *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
	a.mu.Lock()
	a.asked = append(a.asked, req.Params)
	a.mu.Unlock()
	if a.before != nil {
		a.before()
	}
	return a.result, nil
}

func (a *answeringClient) questions() []*mcp.ElicitParams {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]*mcp.ElicitParams(nil), a.asked...)
}

func yes() *mcp.ElicitResult {
	return &mcp.ElicitResult{Action: "accept", Content: map[string]any{confirmField: true}}
}

func sendArgs() map[string]any { return map[string]any{"campaign_id": testCampaignID} }

// answer calls send_campaign again with an answer and a request state.
func answer(t *testing.T, cs *mcp.ClientSession, result *mcp.ElicitResult, state string) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:           toolSendCampaign,
		Arguments:      sendArgs(),
		InputResponses: mcp.InputResponseMap{confirmInputID: result},
		RequestState:   state,
	})
	require.NoError(t, err)
	return res
}

// question calls send_campaign without an answer and returns the question.
func question(t *testing.T, cs *mcp.ClientSession) (*mcp.ElicitParams, string) {
	t.Helper()
	res := callTool(t, cs, toolSendCampaign, sendArgs())
	require.True(t, res.NeedsInput(), "expected a question, got %s", resultText(res))
	params, ok := res.InputRequests[confirmInputID].(*mcp.ElicitParams)
	require.True(t, ok, "input request %T", res.InputRequests[confirmInputID])
	require.NotEmpty(t, res.RequestState)
	return params, res.RequestState
}

func TestSendCampaignAsksWithTheCampaignAndASignedState(t *testing.T) {
	cf := newCampaignFake(t)
	rec := &fakeRecorder{}
	opts := testOptions(t, cf.fakeAPI)
	opts.Recorder = rec
	cs := connectWith(t, opts, manualClient(t), protocol20260728)

	params, signed := question(t, cs)

	assert.Equal(t, "form", params.Mode)
	for _, want := range []string{
		"Send this campaign now to 1545 contacts on 2 lists?\nCampaign: \"October news\"\nSubject: \"Our October news\"\n",
		`- "Newsletter": 1200 contacts`,
		`- "Customers": 345 contacts`,
		`Campaign: "October news"`,
		`Subject: "Our October news"`,
		`Sender: "Example News <news@mail.example.com>"`,
	} {
		assert.Contains(t, params.Message, want)
	}
	schema, err := json.Marshal(params.RequestedSchema)
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"object","required":["send"],"properties":{"send":{"type":"boolean",`+
		`"title":"Send the campaign","description":"Yes to send the campaign as shown, no to cancel."}}}`, string(schema))

	state, ok := openState(opts.ConfirmationKey, signed, time.Now())
	require.True(t, ok, "the state is signed with the confirmation key")
	assert.Equal(t, toolSendCampaign, state.Tool)
	assert.Equal(t, testAccountID.String(), state.AccountID)
	assert.Equal(t, testCampaignID, state.CampaignID)
	assert.Empty(t, state.SendAt)
	assert.Equal(t, "2026-10-02T09:30:00.123456Z", state.UpdatedAt)
	assert.Equal(t, []string{testListID, testList2ID}, state.ListIDs)
	assert.Len(t, state.ContentHash, 64)
	assert.WithinDuration(t, time.Now().Add(confirmationTTL), time.Unix(state.ExpiresAt, 0), 5*time.Second)
	_, ok = openState(make([]byte, 32), signed, time.Now().Add(confirmationTTL+time.Second))
	assert.False(t, ok, "the state expires")

	assert.Empty(t, cf.sends())
	call := rec.only(t)
	assert.True(t, call.Success, "the question round is a success")
	assert.Empty(t, call.ErrorKind)
}

func TestSendCampaignSendsOnceOnYes(t *testing.T) {
	cf := newCampaignFake(t)
	rec := &fakeRecorder{}
	opts := testOptions(t, cf.fakeAPI)
	opts.Recorder = rec
	cs := connectWith(t, opts, manualClient(t), protocol20260728)

	_, state := question(t, cs)
	out := decode[campaignSendOutput](t, answer(t, cs, yes(), state))

	assert.True(t, out.Sent)
	assert.Equal(t, "validating", out.Status)
	sends := cf.sends()
	require.Len(t, sends, 1)
	assert.Empty(t, sends[0].Body, "a send now carries no send_at")

	calls := rec.all()
	require.Len(t, calls, 2)
	for _, call := range calls {
		assert.True(t, call.Success)
		assert.Empty(t, call.ErrorKind)
	}
}

func TestSendCampaignSendsNothingWithoutYes(t *testing.T) {
	tests := []struct {
		name   string
		result *mcp.ElicitResult
	}{
		{"accepted no", &mcp.ElicitResult{Action: "accept", Content: map[string]any{confirmField: false}}},
		{"declined form", &mcp.ElicitResult{Action: "decline"}},
		{"cancelled form", &mcp.ElicitResult{Action: "cancel"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cf := newCampaignFake(t)
			rec := &fakeRecorder{}
			opts := testOptions(t, cf.fakeAPI)
			opts.Recorder = rec
			cs := connectWith(t, opts, manualClient(t), protocol20260728)

			_, state := question(t, cs)
			text := requireToolError(t, answer(t, cs, tt.result, state))

			assert.Contains(t, text, "nothing was sent")
			assert.Empty(t, cf.sends())
			calls := rec.all()
			require.Len(t, calls, 2)
			assert.Empty(t, calls[0].ErrorKind)
			assert.False(t, calls[1].Success)
			assert.Equal(t, ErrorKindDeclined, calls[1].ErrorKind)
		})
	}
}

func TestSendCampaignAsksAgainWithoutAValidAnswer(t *testing.T) {
	otherID := uuid.MustParse("00000000-1111-4222-8333-444444444444").String()
	resign := func(change func(*confirmState)) func(t *testing.T, key []byte, s confirmState, _ *campaignFake) string {
		return func(t *testing.T, key []byte, s confirmState, _ *campaignFake) string {
			change(&s)
			signed, err := signState(key, s)
			require.NoError(t, err)
			return signed
		}
	}
	tests := []struct {
		name  string
		state func(t *testing.T, key []byte, s confirmState, cf *campaignFake) string
	}{
		{"no request state", func(*testing.T, []byte, confirmState, *campaignFake) string { return "" }},
		{"bad signature", func(t *testing.T, _ []byte, s confirmState, _ *campaignFake) string {
			signed, err := signState([]byte("another key of at least 32 bytes!"), s)
			require.NoError(t, err)
			return signed
		}},
		{"expired state", resign(func(s *confirmState) { s.ExpiresAt = time.Now().Add(-time.Minute).Unix() })},
		{"another campaign", resign(func(s *confirmState) { s.CampaignID = otherID })},
		{"another account", resign(func(s *confirmState) { s.AccountID = otherID })},
		{"another tool", resign(func(s *confirmState) { s.Tool = toolScheduleCampaign })},
		{"another send_at", resign(func(s *confirmState) { s.SendAt = "2099-11-01T10:00:00Z" })},
		{"updated_at changed", func(t *testing.T, key []byte, s confirmState, cf *campaignFake) string {
			cf.set("updated_at", "2026-10-02T09:31:00Z")
			return resign(func(*confirmState) {})(t, key, s, cf)
		}},
		{"list_ids changed", func(t *testing.T, key []byte, s confirmState, cf *campaignFake) string {
			cf.set("list_ids", []string{testListID})
			return resign(func(*confirmState) {})(t, key, s, cf)
		}},
		{"segment_ids changed", func(t *testing.T, key []byte, s confirmState, cf *campaignFake) string {
			cf.set("segment_ids", []string{testSegmentID})
			return resign(func(*confirmState) {})(t, key, s, cf)
		}},
		{"exclude_list_ids changed", func(t *testing.T, key []byte, s confirmState, cf *campaignFake) string {
			cf.set("exclude_list_ids", []string{testList2ID})
			return resign(func(*confirmState) {})(t, key, s, cf)
		}},
		{"exclude_segment_ids changed", func(t *testing.T, key []byte, s confirmState, cf *campaignFake) string {
			cf.set("exclude_segment_ids", []string{testSegment2ID})
			return resign(func(*confirmState) {})(t, key, s, cf)
		}},
		{"content changed", func(t *testing.T, key []byte, s confirmState, cf *campaignFake) string {
			cf.set("content", map[string]any{"html": "<p>Changed {{ unsubscribe_url }}</p>"})
			return resign(func(*confirmState) {})(t, key, s, cf)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cf := newCampaignFake(t)
			opts := testOptions(t, cf.fakeAPI)
			cs := connectWith(t, opts, manualClient(t), protocol20260728)

			_, signed := question(t, cs)
			state, ok := openState(opts.ConfirmationKey, signed, time.Now())
			require.True(t, ok)

			res := answer(t, cs, yes(), tt.state(t, opts.ConfirmationKey, *state, cf))

			assert.True(t, res.NeedsInput(), "asks again, got %s", resultText(res))
			assert.NotNil(t, res.InputRequests[confirmInputID])
			assert.Empty(t, cf.sends())
		})
	}
}

func TestSendCampaignWithAnAnsweringClient(t *testing.T) {
	cf := newCampaignFake(t)
	rec := &fakeRecorder{}
	opts := testOptions(t, cf.fakeAPI)
	opts.Recorder = rec
	client := &answeringClient{result: yes()}
	cs := connectWith(t, opts, client.options(), protocol20260728)

	out := decode[campaignSendOutput](t, callTool(t, cs, toolSendCampaign, sendArgs()))

	assert.True(t, out.Sent)
	assert.Len(t, client.questions(), 1)
	assert.Len(t, cf.sends(), 1)
	assert.Len(t, rec.all(), 2, "the question round and the answer round")
}

func TestScheduleCampaignSendsSendAtAfterConfirmation(t *testing.T) {
	cf := newCampaignFake(t)
	client := &answeringClient{result: yes()}
	cs := connectWith(t, testOptions(t, cf.fakeAPI), client.options(), protocol20260728)

	out := decode[campaignSendOutput](t, callTool(t, cs, toolScheduleCampaign, map[string]any{
		"campaign_id": testCampaignID,
		"send_at":     "2099-11-01T11:00:00+01:00",
	}))

	assert.True(t, out.Sent)
	require.NotNil(t, out.SendAt)
	assert.True(t, out.SendAt.Equal(time.Date(2099, 11, 1, 10, 0, 0, 0, time.UTC)))
	questions := client.questions()
	require.Len(t, questions, 1)
	assert.True(t, strings.HasPrefix(questions[0].Message, "Schedule this campaign for Sun, 01 Nov 2099 10:00 UTC, to 1545 contacts on 2 lists?\n"), questions[0].Message)
	sends := cf.sends()
	require.Len(t, sends, 1)
	assert.Equal(t, map[string]any{"send_at": "2099-11-01T10:00:00Z"}, sends[0].Body)
}

func TestScheduleCampaignWithoutYesSendsNothing(t *testing.T) {
	cf := newCampaignFake(t)
	client := &answeringClient{result: &mcp.ElicitResult{Action: "decline"}}
	cs := connectWith(t, testOptions(t, cf.fakeAPI), client.options(), protocol20260728)

	requireToolError(t, callTool(t, cs, toolScheduleCampaign, map[string]any{
		"campaign_id": testCampaignID, "send_at": "2099-11-01T10:00:00Z",
	}))
	assert.Empty(t, cf.sends())
}

func TestSendCampaignByClientKind(t *testing.T) {
	dashboardURL := "https://dash.example.com/account/" + testAccountID.String() + "/marketing/campaigns/" + testCampaignID
	tests := []struct {
		name         string
		version      string
		stateless    bool
		capabilities *mcp.ClientCapabilities
		elicits      bool // the client has an elicitation handler
		wantAsked    bool
	}{
		{name: "2026-07-28 without elicitation", version: protocol20260728},
		{
			name: "2026-07-28 with url elicitation only", version: protocol20260728, elicits: true,
			capabilities: &mcp.ClientCapabilities{Elicitation: &mcp.ElicitationCapabilities{URL: &mcp.URLElicitationCapabilities{}}},
		},
		{name: "2026-07-28 with form elicitation, stateless", version: protocol20260728, stateless: true, elicits: true, wantAsked: true},
		{name: "2025-11-25 with elicitation, stateless", version: protocol20251125, stateless: true, elicits: true},
		{name: "2025-11-25 with elicitation, not stateless", version: protocol20251125, elicits: true, wantAsked: true},
		{
			name: "2025-06-18 with empty elicitation, not stateless", version: protocol20250618, elicits: true, wantAsked: true,
			capabilities: &mcp.ClientCapabilities{Elicitation: &mcp.ElicitationCapabilities{}},
		},
		{name: "2025-06-18 without elicitation, not stateless", version: protocol20250618},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cf := newCampaignFake(t)
			rec := &fakeRecorder{}
			opts := testOptions(t, cf.fakeAPI)
			opts.Stateless = tt.stateless
			opts.Recorder = rec
			opts.DashboardBaseURL = "https://dash.example.com/"
			client := &answeringClient{result: yes()}
			clientOpts := &mcp.ClientOptions{Capabilities: tt.capabilities}
			if tt.elicits {
				clientOpts.ElicitationHandler = client.handle
			}
			cs := connectWith(t, opts, clientOpts, tt.version)
			require.Equal(t, tt.version, cs.InitializeResult().ProtocolVersion)

			out := decode[campaignSendOutput](t, callTool(t, cs, toolSendCampaign, sendArgs()))

			if tt.wantAsked {
				assert.True(t, out.Sent)
				assert.Len(t, client.questions(), 1)
				assert.Len(t, cf.sends(), 1)
				return
			}
			assert.False(t, out.Sent)
			assert.Equal(t, dashboardURL, out.DashboardURL)
			assert.Empty(t, client.questions())
			assert.Empty(t, cf.calls(), "nothing is read or sent")
			call := rec.only(t)
			assert.True(t, call.Success, "the dashboard URL result is a success")
			assert.Empty(t, call.ErrorKind)
		})
	}
}

func TestSendCampaignOnAnOlderProtocolRecordsANoOnce(t *testing.T) {
	cf := newCampaignFake(t)
	rec := &fakeRecorder{}
	opts := testOptions(t, cf.fakeAPI)
	opts.Recorder = rec
	client := &answeringClient{result: &mcp.ElicitResult{Action: "decline"}}
	cs := connectWith(t, opts, client.options(), protocol20251125)

	requireToolError(t, callTool(t, cs, toolSendCampaign, sendArgs()))

	assert.Len(t, client.questions(), 1)
	assert.Empty(t, cf.sends())
	assert.Equal(t, ErrorKindDeclined, rec.only(t).ErrorKind)
}

func TestSendCampaignOnAnOlderProtocolSendsNothingWhenTheCampaignChangesWhileAsked(t *testing.T) {
	cf := newCampaignFake(t)
	client := &answeringClient{result: yes()}
	client.before = func() { cf.set("updated_at", "2026-10-02T09:45:00Z") }
	cs := connectWith(t, testOptions(t, cf.fakeAPI), client.options(), protocol20251125)

	out := decode[campaignSendOutput](t, callTool(t, cs, toolSendCampaign, sendArgs()))

	assert.False(t, out.Sent)
	assert.Contains(t, out.Message, "the answer did not match the question")
	assert.Contains(t, out.Message, "Call send_campaign again")
	assert.Empty(t, cf.sends())
}

func TestSendCampaignWithUnknownScopesAndAForbiddenRead(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"list read", "/lists/" + testList2ID},
		{"campaign read", "/campaigns/" + testCampaignID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeAPI(t)
			reads := map[string]any{
				"/campaigns/" + testCampaignID: campaignJSON(),
				"/lists/" + testListID:         listJSON(testListID, "Newsletter", 1200),
				"/lists/" + testList2ID:        listJSON(testList2ID, "Customers", 345),
			}
			for path, body := range reads {
				if path == tt.path {
					f.handle("GET "+apiPath(path), http.StatusForbidden, map[string]any{"message": "api key does not have sufficient permissions"})
					continue
				}
				f.handle("GET "+apiPath(path), http.StatusOK, body)
			}
			f.handle("POST "+apiPath("/campaigns/"+testCampaignID+"/send"), http.StatusOK, campaignJSON())
			rec := &fakeRecorder{}
			opts := testOptions(t, f)
			opts.Scopes = nil
			opts.Recorder = rec
			client := &answeringClient{result: yes()}
			cs := connectWith(t, opts, client.options(), protocol20260728)

			out := decode[campaignSendOutput](t, callTool(t, cs, toolSendCampaign, sendArgs()))

			assert.False(t, out.Sent)
			assert.Equal(t, "https://dash.example.com/account/"+testAccountID.String()+"/marketing/campaigns/"+testCampaignID, out.DashboardURL)
			assert.Empty(t, client.questions())
			for _, c := range f.calls() {
				assert.NotEqual(t, http.MethodPost, c.Method, "nothing is sent")
			}
			assert.Empty(t, rec.only(t).ErrorKind)
		})
	}
}

func TestSendCampaignWithKnownScopesAndAForbiddenReadIsAnError(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("GET "+apiPath("/campaigns/"+testCampaignID), http.StatusForbidden, map[string]any{"message": "forbidden"})
	opts := testOptions(t, f)
	opts.Scopes = []string{"campaigns:send", "campaigns:read", "lists:read"}
	client := &answeringClient{result: yes()}
	cs := connectWith(t, opts, client.options(), protocol20260728)

	text := requireToolError(t, callTool(t, cs, toolSendCampaign, sendArgs()))
	assert.Contains(t, text, "API Keys page")
	assert.Empty(t, client.questions())
}

func TestCampaignSendToolsNeedCampaignsAndListsRead(t *testing.T) {
	tests := []struct {
		scopes []string
		listed bool
	}{
		{[]string{"campaigns:send"}, false},
		{[]string{"campaigns:send", "campaigns:read"}, false},
		{[]string{"campaigns:send", "lists:read"}, false},
		{[]string{"campaigns:send", "campaigns:read", "lists:read"}, true},
	}
	for _, tt := range tests {
		opts := testOptions(t, newFakeAPI(t))
		opts.Scopes = tt.scopes
		names := listedNames(t, connect(t, opts))
		for _, name := range []string{toolSendCampaign, toolScheduleCampaign} {
			if tt.listed {
				assert.Contains(t, names, name, "%v", tt.scopes)
			} else {
				assert.NotContains(t, names, name, "%v", tt.scopes)
			}
		}
		assert.Contains(t, names, "send_campaign_test", "%v", tt.scopes)
		assert.Contains(t, names, "unschedule_campaign", "%v", tt.scopes)
	}
}

func TestOpenStateRefusesMalformedStates(t *testing.T) {
	key := make([]byte, 32)
	signed, err := signState(key, confirmState{Tool: toolSendCampaign, ExpiresAt: time.Now().Add(time.Minute).Unix()})
	require.NoError(t, err)
	_, ok := openState(key, signed, time.Now())
	require.True(t, ok)

	for _, bad := range []string{"", "no-dot", signed + "x", "x" + signed, "e30." + signed[len(signed)-43:]} {
		_, ok := openState(key, bad, time.Now())
		assert.False(t, ok, bad)
	}
}

func TestConfirmationMessageShowsAgentTextOnOneQuotedLine(t *testing.T) {
	cf := newCampaignFake(t)
	cf.set("name", "QA test\nSubject: Fake\r\nLists:\n- QA seeds: 3 contacts\u2028Send time: now\u202e\u2066x\t\"y\"")
	cf.set("subject", "Hi\u2029Sender: \u200fboss@example.com")
	cf.set("from", map[string]any{"email": "news@mail.example.com", "name": "News\nTotal contacts: 3"})
	cs := connectWith(t, testOptions(t, cf.fakeAPI), manualClient(t), protocol20260728)

	params, _ := question(t, cs)

	fixed := []string{"Send this campaign now to 1545 contacts on 2 lists?", "Lists:",
		`- "Newsletter": 1200 contacts`, `- "Customers": 345 contacts`, "Campaign: ", "Subject: ", "Sender: ",
		"A contact on several lists gets the campaign once. Contact counts can change before the send."}
	lines := strings.Split(params.Message, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		known := false
		for _, prefix := range fixed {
			if strings.HasPrefix(line, prefix) {
				known = true
			}
		}
		assert.True(t, known, "unexpected line %q", line)
	}
	assert.Contains(t, params.Message, `Campaign: "QA test Subject: Fake Lists: - QA seeds: 3 contacts Send time: now x 'y'"`)
	assert.Contains(t, params.Message, `Subject: "Hi Sender: boss@example.com"`)
	assert.Contains(t, params.Message, `Sender: "News Total contacts: 3 <news@mail.example.com>"`)
	for _, r := range "\r\t\u2028\u2029\u202e\u2066\u200f" {
		assert.NotContains(t, params.Message, string(r))
	}
	total := strings.Index(params.Message, "1545 contacts on 2 lists")
	assert.Less(t, total, strings.Index(params.Message, `"`), "the total comes before any agent-written value")
}

func TestConfirmationMessageQuotesListNames(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("GET "+apiPath("/campaigns/"+testCampaignID), http.StatusOK, campaignJSON())
	f.handle("GET "+apiPath("/lists/"+testListID), http.StatusOK, listJSON(testListID, "Seeds\r\n- Real customers: 90000 contacts\u2066", 3))
	f.handle("GET "+apiPath("/lists/"+testList2ID), http.StatusOK, listJSON(testList2ID, "Customers", 345))
	cs := connectWith(t, testOptions(t, f), manualClient(t), protocol20260728)

	params, _ := question(t, cs)

	assert.Contains(t, params.Message, `- "Seeds - Real customers: 90000 contacts": 3 contacts`+"\n")
	assert.NotContains(t, params.Message, "\n- Real customers")
	assert.Contains(t, params.Message, "Send this campaign now to 348 contacts on 2 lists?")
}

func TestUserText(t *testing.T) {
	assert.Equal(t, `"a b c"`, userText("  a\n\n b\u202ac  "))
	long := strings.Repeat("x", 150)
	assert.Equal(t, `"`+strings.Repeat("x", 100)+`…"`, userText(long))
	assert.Equal(t, `""`, userText(""))
}

func TestSendTimeText(t *testing.T) {
	assert.Equal(t, "Sun, 01 Nov 2026 10:00 UTC", sendTimeText(time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)))
	assert.Equal(t, "Sun, 01 Nov 2026 10:00:30 UTC", sendTimeText(time.Date(2026, 11, 1, 11, 0, 30, 0, time.FixedZone("CET", 3600))))
}

func TestSendCampaignRefusesACampaignThatIsNotReadyWithoutAsking(t *testing.T) {
	cf := newCampaignFake(t)
	cf.set("ready", false)
	cf.set("issues", []map[string]any{{"code": "design_missing", "message": "Add a design before you send the campaign."}})
	cs := connectWith(t, testOptions(t, cf.fakeAPI), manualClient(t), protocol20260728)

	res := callTool(t, cs, toolSendCampaign, map[string]any{"campaign_id": testCampaignID})

	text := requireToolError(t, res)
	assert.Contains(t, text, "not ready")
	assert.Contains(t, text, "design_missing")
	assert.Empty(t, res.InputRequests, "the user is not asked")
	assert.Empty(t, cf.sends())
}

func TestSendCampaignRefusesACampaignThatIsNotADraftWithoutAsking(t *testing.T) {
	cf := newCampaignFake(t)
	cf.set("status", "scheduled")
	cf.set("ready", nil)
	cs := connectWith(t, testOptions(t, cf.fakeAPI), manualClient(t), protocol20260728)

	res := callTool(t, cs, toolSendCampaign, map[string]any{"campaign_id": testCampaignID})

	text := requireToolError(t, res)
	assert.Contains(t, text, "scheduled")
	assert.Contains(t, text, "only a draft")
	assert.Empty(t, res.InputRequests, "the user is not asked")
	assert.Empty(t, cf.sends())
}

func TestScheduleCampaignRefusesASendTimeTooSoon(t *testing.T) {
	cf := newCampaignFake(t)
	cs := connectWith(t, testOptions(t, cf.fakeAPI), manualClient(t), protocol20260728)

	res := callTool(t, cs, toolScheduleCampaign, map[string]any{
		"campaign_id": testCampaignID,
		"send_at":     time.Now().Add(30 * time.Second).UTC().Format(time.RFC3339),
	})

	assert.Contains(t, requireToolError(t, res), "at least a minute")
	assert.Empty(t, cf.calls(), "no API call and no question")
}

func TestSendCampaignRefusesListsWithoutReachableContacts(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("GET "+apiPath("/campaigns/"+testCampaignID), http.StatusOK, campaignJSON())
	f.handle("GET "+apiPath("/lists/"+testListID), http.StatusOK, listJSON(testListID, "Newsletter", 0))
	f.handle("GET "+apiPath("/lists/"+testList2ID), http.StatusOK, listJSON(testList2ID, "Customers", 0))
	rec := &fakeRecorder{}
	opts := testOptions(t, f)
	opts.Recorder = rec
	client := &answeringClient{result: yes()}
	cs := connectWith(t, opts, client.options(), protocol20260728)

	text := requireToolError(t, callTool(t, cs, toolSendCampaign, sendArgs()))

	assert.Contains(t, text, "no reachable contacts")
	assert.Empty(t, client.questions(), "the user is not asked")
	for _, c := range f.calls() {
		assert.NotEqual(t, http.MethodPost, c.Method, "nothing is sent")
	}
	assert.Equal(t, ErrorKindInvalidInput, rec.only(t).ErrorKind)
}

// setSegmentAudience gives the campaign one list and one segment, and
// excludes the other list and segment.
func (cf *campaignFake) setSegmentAudience() {
	cf.set("list_ids", []string{testListID})
	cf.set("segment_ids", []string{testSegmentID})
	cf.set("exclude_list_ids", []string{testList2ID})
	cf.set("exclude_segment_ids", []string{testSegment2ID})
}

func TestSendCampaignAsksWithTheSegmentsAndExclusions(t *testing.T) {
	cf := newCampaignFake(t)
	cf.setSegmentAudience()
	opts := testOptions(t, cf.fakeAPI)
	cs := connectWith(t, opts, manualClient(t), protocol20260728)

	params, signed := question(t, cs)

	for _, want := range []string{
		"Send this campaign now to 1580 contacts on 1 list and 1 segment?\n",
		"Lists:\n- \"Newsletter\": 1200 contacts\n",
		"Segments:\n- \"Engaged\": 380 contacts\n",
		"Excluded lists:\n- \"Customers\"\n",
		"Excluded segments:\n- \"Lapsed\"\n",
		"a contact on an excluded list or segment gets none",
	} {
		assert.Contains(t, params.Message, want)
	}
	state, ok := openState(opts.ConfirmationKey, signed, time.Now())
	require.True(t, ok)
	assert.Equal(t, []string{testListID}, state.ListIDs)
	assert.Equal(t, []string{testSegmentID}, state.SegmentIDs)
	assert.Equal(t, []string{testList2ID}, state.ExcludeListIDs)
	assert.Equal(t, []string{testSegment2ID}, state.ExcludeSegmentIDs)

	out := decode[campaignSendOutput](t, answer(t, cs, yes(), signed))
	assert.True(t, out.Sent)
	assert.Len(t, cf.sends(), 1)
}

func TestSendCampaignToASegmentNotCounted(t *testing.T) {
	cf := newCampaignFake(t)
	cf.setSegment(testSegment2ID, segmentJSON(testSegment2ID, "Lapsed", "ready", nil))
	cf.set("list_ids", []string{})
	cf.set("segment_ids", []string{testSegment2ID})
	cs := connectWith(t, testOptions(t, cf.fakeAPI), manualClient(t), protocol20260728)

	params, _ := question(t, cs)

	assert.True(t, strings.HasPrefix(params.Message, "Send this campaign now to the contacts of 1 segment?\n"), params.Message)
	assert.Contains(t, params.Message, "Segments:\n- \"Lapsed\": not counted\n")
	assert.NotContains(t, params.Message, "Lists:")
}

func TestSendCampaignRefusesASegmentThatIsNotReadyWithoutAsking(t *testing.T) {
	tests := []struct {
		name     string
		segment  map[string]any
		excluded bool
		want     string
	}{
		{"building", segmentJSON(testSegmentID, "Engaged", "building", nil), false, "still finding its members"},
		{"building exclusion", segmentJSON(testSegmentID, "Engaged", "building", nil), true, "still finding its members"},
		{"failed", func() map[string]any {
			s := segmentJSON(testSegmentID, "Engaged", "failed", nil)
			s["failure_reason"] = "The list in conditions[0] was deleted."
			return s
		}(), false, "has failed (The list in conditions[0] was deleted)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeAPI(t)
			c := campaignJSON()
			c["list_ids"] = []string{testListID}
			if tt.excluded {
				c["exclude_segment_ids"] = []string{testSegmentID}
			} else {
				c["segment_ids"] = []string{testSegmentID}
			}
			f.handle("GET "+apiPath("/campaigns/"+testCampaignID), http.StatusOK, c)
			f.handle("GET "+apiPath("/lists/"+testListID), http.StatusOK, listJSON(testListID, "Newsletter", 1200))
			f.handle("GET "+apiPath("/segments/"+testSegmentID), http.StatusOK, tt.segment)
			rec := &fakeRecorder{}
			opts := testOptions(t, f)
			opts.Recorder = rec
			client := &answeringClient{result: yes()}
			cs := connectWith(t, opts, client.options(), protocol20260728)

			text := requireToolError(t, callTool(t, cs, toolSendCampaign, sendArgs()))

			assert.Contains(t, text, `segment "Engaged"`)
			assert.Contains(t, text, tt.want)
			assert.Empty(t, client.questions(), "the user is not asked")
			for _, c := range f.calls() {
				assert.NotEqual(t, http.MethodPost, c.Method, "nothing is sent")
			}
			assert.Equal(t, ErrorKindInvalidInput, rec.only(t).ErrorKind)
		})
	}
}

func TestSendCampaignRefusesSegmentsWithoutReachableContacts(t *testing.T) {
	cf := newCampaignFake(t)
	cf.setSegment(testSegment2ID, segmentJSON(testSegment2ID, "Lapsed", "ready", 0))
	cf.set("list_ids", []string{})
	cf.set("segment_ids", []string{testSegment2ID})
	client := &answeringClient{result: yes()}
	cs := connectWith(t, testOptions(t, cf.fakeAPI), client.options(), protocol20260728)

	text := requireToolError(t, callTool(t, cs, toolSendCampaign, sendArgs()))

	assert.Contains(t, text, "The campaign's segments have no reachable contacts")
	assert.Empty(t, client.questions())
	assert.Empty(t, cf.sends())
}

func TestSendCampaignWithSegmentsAndKnownScopesWithoutSegmentsRead(t *testing.T) {
	cf := newCampaignFake(t)
	cf.setSegmentAudience()
	opts := testOptions(t, cf.fakeAPI)
	opts.Scopes = []string{"campaigns:send", "campaigns:read", "lists:read"}
	client := &answeringClient{result: yes()}
	cs := connectWith(t, opts, client.options(), protocol20260728)

	out := decode[campaignSendOutput](t, callTool(t, cs, toolSendCampaign, sendArgs()))

	assert.False(t, out.Sent)
	assert.Contains(t, out.Message, "may not read the campaign's segments")
	assert.NotEmpty(t, out.DashboardURL)
	assert.Empty(t, client.questions())
	for _, c := range cf.calls() {
		assert.NotContains(t, c.Path, "/segments/", "no segment is read")
	}
	assert.Empty(t, cf.sends())
}

func TestSendCampaignWithUnknownScopesAndAForbiddenSegmentRead(t *testing.T) {
	f := newFakeAPI(t)
	c := campaignJSON()
	c["segment_ids"] = []string{testSegmentID}
	f.handle("GET "+apiPath("/campaigns/"+testCampaignID), http.StatusOK, c)
	f.handle("GET "+apiPath("/lists/"+testListID), http.StatusOK, listJSON(testListID, "Newsletter", 1200))
	f.handle("GET "+apiPath("/lists/"+testList2ID), http.StatusOK, listJSON(testList2ID, "Customers", 345))
	f.handle("GET "+apiPath("/segments/"+testSegmentID), http.StatusForbidden, map[string]any{"message": "api key does not have sufficient permissions"})
	client := &answeringClient{result: yes()}
	cs := connectWith(t, testOptions(t, f), client.options(), protocol20260728)

	out := decode[campaignSendOutput](t, callTool(t, cs, toolSendCampaign, sendArgs()))

	assert.False(t, out.Sent)
	assert.Contains(t, out.Message, "lists and segments")
	assert.NotEmpty(t, out.DashboardURL)
	assert.Empty(t, client.questions())
}

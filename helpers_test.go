package ahasendmcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testAccountID = uuid.MustParse("6b1e1f43-5d2c-4b0e-9a57-0c1f2a3b4c5d")

// apiPath returns a v2 path under the test account.
func apiPath(suffix string) string {
	return "/v2/accounts/" + testAccountID.String() + suffix
}

// recordedRequest is one request the fake API received.
type recordedRequest struct {
	Method string
	Path   string
	Query  url.Values
	Body   map[string]any
	Header http.Header
}

// fakeAPI is an httptest fake of the v2 API. Routes answer with a fixed
// status and JSON body; every request is recorded.
type fakeAPI struct {
	srv *httptest.Server
	mux *http.ServeMux

	mu       sync.Mutex
	requests []recordedRequest
}

func newFakeAPI(t *testing.T) *fakeAPI {
	t.Helper()
	f := &fakeAPI{mux: http.NewServeMux()}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeAPI) serve(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	rec := recordedRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(), Header: r.Header.Clone()}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &rec.Body)
	}
	f.mu.Lock()
	f.requests = append(f.requests, rec)
	f.mu.Unlock()
	f.mux.ServeHTTP(w, r)
}

// handle answers requests matching pattern ("GET /v2/...") with status and
// body encoded as JSON.
func (f *fakeAPI) handle(pattern string, status int, body any) {
	f.mux.HandleFunc(pattern, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	})
}

// handleFunc answers requests matching pattern with the status and body fn
// returns for each request.
func (f *fakeAPI) handleFunc(pattern string, fn func(r *http.Request) (int, any)) {
	f.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		status, body := fn(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	})
}

func (f *fakeAPI) calls() []recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recordedRequest(nil), f.requests...)
}

// onlyCall returns the one request the fake received.
func (f *fakeAPI) onlyCall(t *testing.T) recordedRequest {
	t.Helper()
	calls := f.calls()
	require.Len(t, calls, 1, "API requests")
	return calls[0]
}

// testClient returns an ahasend-go client for baseURL, without retries or
// client-side rate limits.
func testClient(t *testing.T, baseURL string) *api.APIClient {
	t.Helper()
	u, err := url.Parse(baseURL)
	require.NoError(t, err)
	return api.NewAPIClient(
		api.WithAPIKey("aha-sk-test"),
		api.WithRateLimit(false),
		api.WithRetryConfig(api.RetryConfig{Enabled: false}),
		api.WithHTTPClient(&http.Client{Timeout: 5 * time.Second}),
		func(cfg *api.Configuration) {
			cfg.Scheme = u.Scheme
			cfg.Host = u.Host
		},
	)
}

// testOptions returns valid Options against the fake API, listing every
// tool.
func testOptions(t *testing.T, f *fakeAPI) Options {
	t.Helper()
	return Options{
		Client:           testClient(t, f.srv.URL),
		AccountID:        testAccountID,
		DashboardBaseURL: "https://dash.example.com",
		ConfirmationKey:  make([]byte, 32),
	}
}

// connect builds a server from opts and connects a real MCP client to it
// over the in-memory transport.
func connect(t *testing.T, opts Options) *mcp.ClientSession {
	t.Helper()
	return connectWith(t, opts, nil, "")
}

// connectWith connects a client with clientOpts on protocol version (the
// latest when empty).
func connectWith(t *testing.T, opts Options, clientOpts *mcp.ClientOptions, version string) *mcp.ClientSession {
	t.Helper()
	server, err := NewServer(&mcp.Implementation{Name: "ahasend-test", Version: "test"}, opts)
	require.NoError(t, err)

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, clientOpts)
	session, err := client.Connect(ctx, clientTransport, &mcp.ClientSessionOptions{ProtocolVersion: version})
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

// callTool calls a tool and fails the test on a protocol error.
func callTool(t *testing.T, cs *mcp.ClientSession, name string, args any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	require.NoError(t, err, "calling %s", name)
	return res
}

// decode reads a successful tool result's structured content into T.
func decode[T any](t *testing.T, res *mcp.CallToolResult) T {
	t.Helper()
	require.False(t, res.IsError, "tool error: %s", resultText(res))
	raw, err := json.Marshal(res.StructuredContent)
	require.NoError(t, err)
	var out T
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

// resultText joins a result's text content.
func resultText(res *mcp.CallToolResult) string {
	var parts []string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// requireToolError asserts a tool error result and returns its text.
func requireToolError(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	require.True(t, res.IsError, "expected a tool error, got %s", resultText(res))
	return resultText(res)
}

// listedNames lists the tool names a session sees.
func listedNames(t *testing.T, cs *mcp.ClientSession) []string {
	t.Helper()
	res, err := cs.ListTools(context.Background(), nil)
	require.NoError(t, err)
	names := make([]string, 0, len(res.Tools))
	for _, tl := range res.Tools {
		names = append(names, tl.Name)
	}
	return names
}

// toolNames lists the catalogue's tool names, optionally only one toolset's.
func toolNames(toolset string) []string {
	var names []string
	for _, info := range Catalogue() {
		if toolset == "" || info.Toolset == toolset {
			names = append(names, info.Name)
		}
	}
	return names
}

// fakeRecorder keeps every ToolCall.
type fakeRecorder struct {
	mu    sync.Mutex
	calls []ToolCall
}

func (r *fakeRecorder) ToolCalled(_ context.Context, call ToolCall) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, call)
}

func (r *fakeRecorder) only(t *testing.T) ToolCall {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	require.Len(t, r.calls, 1, "recorded calls")
	return r.calls[0]
}

// Fixtures.

const (
	outboundMessageID = "0c6c5a8e-3f2b-4d6e-8a1c-1b2c3d4e5f60"
	inboundMessageID  = "9d8e7f60-1a2b-4c3d-8e9f-a0b1c2d3e4f5"
)

func messageJSON(id, direction string) map[string]any {
	return map[string]any{
		"object":       "message",
		"id":           id,
		"message_id":   "<" + id + "@example.com>",
		"created_at":   "2026-10-01T10:00:00Z",
		"updated_at":   "2026-10-01T10:01:00Z",
		"retain_until": "2026-10-31T10:00:00Z",
		"subject":      "Subject of " + direction,
		"tags":         []string{"welcome"},
		"sender":       "app@mail.example.com",
		"recipient":    "user@example.org",
		"direction":    direction,
		"status":       "Delivered",
		"num_attempts": 1,
		"delivery_attempts": []map[string]any{
			{"time": "2026-10-01T10:00:30Z", "log": "250 OK", "status": "Delivered"},
		},
		"domain_id":  "1f2e3d4c-5b6a-4978-8695-a4b3c2d1e0f9",
		"account_id": testAccountID.String(),
	}
}

func messagePage(nextCursor string, messages ...map[string]any) map[string]any {
	pagination := map[string]any{"has_more": nextCursor != ""}
	if nextCursor != "" {
		pagination["next_cursor"] = nextCursor
	}
	return map[string]any{"object": "list", "data": messages, "pagination": pagination}
}

func domainJSON(name string) map[string]any {
	return map[string]any{
		"object":     "domain",
		"id":         "1f2e3d4c-5b6a-4978-8695-a4b3c2d1e0f9",
		"created_at": "2026-09-01T10:00:00Z",
		"updated_at": "2026-09-01T10:00:00Z",
		"domain":     name,
		"account_id": testAccountID.String(),
		"dns_records": []map[string]any{
			{"type": "TXT", "host": "aha-sel._domainkey." + name, "content": "v=DKIM1; p=abc", "required": true, "propagated": false},
			{"type": "CNAME", "host": "track." + name, "content": "track.ahasend.com", "required": false, "propagated": true},
		},
		"dns_valid":      false,
		"rotation_ready": false,
		"sending_type":   "transactional",
		"paused":         false,
	}
}

func sendResponse(recipients ...string) map[string]any {
	var data []map[string]any
	for i, r := range recipients {
		data = append(data, map[string]any{
			"object":    "message",
			"id":        uuid.NewSHA1(uuid.Nil, []byte{byte(i)}).String(),
			"recipient": map[string]any{"email": r},
			"status":    "queued",
		})
	}
	return map[string]any{"object": "list", "data": data}
}

// IDs of the Stage 2 fixtures.
const (
	testTemplateID         = "3a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d"
	testVersionID          = "4b2c3d4e-5f6a-4b7c-8d9e-0f1a2b3c4d5e"
	testCampaignTemplateID = "5c3d4e5f-6a7b-4c8d-9e0f-1a2b3c4d5e6f"
	testContactID          = "6d4e5f6a-7b8c-4d9e-8f0a-2b3c4d5e6f70"
	testListID             = "7e5f6a7b-8c9d-4e0f-9a1b-3c4d5e6f7081"
	testList2ID            = "8f6a7b8c-9d0e-4f1a-8b2c-4d5e6f708192"
	testCampaignID         = "9a7b8c9d-0e1f-4a2b-9c3d-5e6f708192a3"
	testWebhookID          = "0b8c9d0e-1f2a-4b3c-8d4e-6f708192a3b4"
	testRouteID            = "1c9d0e1f-2a3b-4c4d-9e5f-708192a3b4c5"
	testSMTPCredentialID   = "2d0e1f2a-3b4c-4d5e-8f6a-8192a3b4c5d6"
	testSubAccountID       = "3e1f2a3b-4c5d-4e6f-9a7b-92a3b4c5d6e7"
)

// toolCase is one tool call against the fake API: the arguments, the request
// the tool must send, the API's answer, and a check of the tool's result.
type toolCase struct {
	tool      string
	args      map[string]any
	method    string
	path      string // under the test account
	wantQuery map[string]string
	noQuery   []string       // query parameters that must be absent
	wantBody  map[string]any // nil: no body
	status    int            // 0: 200
	resp      any
	check     func(t *testing.T, out map[string]any)
}

// runToolCases runs each case in a subtest named after its tool.
func runToolCases(t *testing.T, cases []toolCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			f := newFakeAPI(t)
			status := tc.status
			if status == 0 {
				status = http.StatusOK
			}
			f.handle(tc.method+" "+apiPath(tc.path), status, tc.resp)
			cs := connect(t, testOptions(t, f))

			res := callTool(t, cs, tc.tool, tc.args)
			out := decode[map[string]any](t, res)

			req := f.onlyCall(t)
			assert.Equal(t, tc.method, req.Method)
			assert.Equal(t, apiPath(tc.path), req.Path)
			for k, v := range tc.wantQuery {
				assert.Equal(t, v, req.Query.Get(k), "query %s", k)
			}
			for _, k := range tc.noQuery {
				assert.NotContains(t, req.Query, k, "query %s is not sent", k)
			}
			assert.Equal(t, tc.wantBody, req.Body)
			if tc.check != nil {
				tc.check(t, out)
			}
		})
	}
}

// field reads a nested field of a decoded result, such as "from.email" or
// "items.0.id".
func field(t *testing.T, v any, path string) any {
	t.Helper()
	for _, key := range strings.Split(path, ".") {
		switch node := v.(type) {
		case map[string]any:
			v = node[key]
		case []any:
			i, err := strconv.Atoi(key)
			require.NoError(t, err, path)
			require.Less(t, i, len(node), path)
			v = node[i]
		default:
			t.Fatalf("no %s in %v", path, v)
		}
	}
	return v
}

func success(message string) map[string]any { return map[string]any{"message": message} }

func page(items ...map[string]any) map[string]any {
	return map[string]any{"object": "list", "data": items, "pagination": map[string]any{"has_more": true, "next_cursor": "next-page"}}
}

const fixtureTime = "2026-10-01T10:00:00Z"

func templateJSON() map[string]any {
	return map[string]any{
		"object": "template", "id": testTemplateID, "created_at": fixtureTime, "updated_at": fixtureTime,
		"name": "Welcome", "subject": "Hello {{ first_name }}", "preheader": "Glad you are here",
		"variables": []map[string]any{{"name": "first_name", "required": true}},
		"from":      map[string]any{"email": "app@mail.example.com", "name": "App"},
		"reply_to":  map[string]any{"email": "help@example.com"},
		"editor":    "html", "has_draft": true,
		"content": map[string]any{"html": "<p>TEMPLATE-HTML-CONTENT</p>", "text": "TEMPLATE-TEXT-CONTENT", "text_is_custom": false},
	}
}

func campaignTemplateJSON() map[string]any {
	return map[string]any{
		"object": "campaign_template", "id": testCampaignTemplateID, "created_at": fixtureTime, "updated_at": fixtureTime,
		"name": "Newsletter", "editor": "advanced",
		// An advanced design returns its MJML, not the HTML made from it.
		"content": map[string]any{"mjml": "<mjml>CAMPAIGN-TEMPLATE-MJML</mjml>", "text": "CAMPAIGN-TEMPLATE-TEXT"},
	}
}

func contactJSON() map[string]any {
	return map[string]any{
		"object": "contact", "id": testContactID, "created_at": fixtureTime, "updated_at": fixtureTime,
		"email": "ann@example.org", "first_name": "Ann", "last_name": "Lee", "status": "enabled", "status_reason": "",
		"unsubscribed": false, "unsubscribed_at": nil, "attributes": map[string]any{"plan": "pro"},
		"validation_status": "valid", "last_validated_at": nil,
	}
}

func listJSON(id, name string, count int) map[string]any {
	return map[string]any{
		"object": "list", "id": id, "created_at": fixtureTime, "updated_at": fixtureTime,
		"name": name, "description": "Monthly news", "tags": []string{"news"}, "contact_count": count,
	}
}

func membershipJSON() map[string]any {
	return map[string]any{
		"object": "list_contact", "list_id": testListID, "contact_id": testContactID, "email": "ann@example.org",
		"subscription_status": "confirmed", "subscribed_at": fixtureTime, "unsubscribed_at": nil,
		"created_at": fixtureTime, "updated_at": fixtureTime,
		"list": map[string]any{"object": "list", "id": testListID, "name": "Newsletter", "created_at": fixtureTime, "updated_at": fixtureTime},
	}
}

func campaignJSON() map[string]any {
	return map[string]any{
		"object": "campaign", "id": testCampaignID, "created_at": fixtureTime, "updated_at": "2026-10-02T09:30:00.123456Z",
		"name": "October news", "subject": "Our October news", "preheader": "What is new",
		"from": map[string]any{"email": "news@mail.example.com", "name": "Example News"}, "reply_to": "help@example.com",
		"status": "draft", "status_reason": nil, "send_at": nil, "send_resumes_at": nil, "finished_at": nil,
		"list_ids": []string{testListID, testList2ID}, "editor": "html",
		"content": map[string]any{"html": "<p>CAMPAIGN-HTML-CONTENT {{ unsubscribe_url }}</p>", "text": "CAMPAIGN-TEXT"},
		"ready":   true, "issues": []any{},
	}
}

func webhookJSON(secret string) map[string]any {
	w := map[string]any{
		"object": "webhook", "id": testWebhookID, "created_at": fixtureTime, "updated_at": fixtureTime,
		"name": "App events", "url": "https://app.example.com/hooks", "enabled": true,
		"on_delivered": true, "on_bounced": true, "scope": "scoped", "domains": []string{"mail.example.com"},
		"error_count": 2, "success_count": 40, "errors_since_last_success": 0,
	}
	if secret != "" {
		w["secret"] = secret
	}
	return w
}

func routeJSON(secret string) map[string]any {
	r := map[string]any{
		"object": "route", "id": testRouteID, "created_at": fixtureTime, "updated_at": fixtureTime,
		"name": "Support", "url": "https://app.example.com/inbound", "recipient": "support@mail.example.com",
		"attachments": true, "headers": false, "group_by_message_id": false, "strip_replies": true, "enabled": true,
		"error_count": 0, "success_count": 3, "errors_since_last_success": 0,
	}
	if secret != "" {
		r["secret"] = secret
	}
	return r
}

func suppressionJSON() map[string]any {
	return map[string]any{
		"object": "suppression", "id": "4f2a3b4c-5d6e-4f7a-8b8c-a3b4c5d6e7f8", "created_at": fixtureTime,
		"email": "ann@example.org", "expires_at": "2027-01-01T00:00:00Z", "domain": "mail.example.com",
		"reason": "asked to stop", "protected": false,
	}
}

func smtpCredentialJSON() map[string]any {
	return map[string]any{
		"object": "smtp_credential", "id": testSMTPCredentialID, "created_at": fixtureTime, "updated_at": fixtureTime,
		"name": "Rails app", "username": "smtp-user", "password": "SMTP-PASSWORD-SECRET", "sandbox": false,
		"scope": "global", "domains": []string{},
	}
}

func subAccountJSON() map[string]any {
	return map[string]any{
		"object": "sub_account", "id": testSubAccountID, "parent_account_id": testAccountID.String(),
		"created_at": fixtureTime, "name": "Client A", "website": "https://client-a.example", "status": "active",
		"monthly_credit": 5000, "domain_count": 2, "member_count": 1, "last_activity_at": nil,
	}
}

// inputProperties returns each listed tool's input property descriptions, by
// tool and property name.
func inputProperties(t *testing.T, cs *mcp.ClientSession) map[string]map[string]string {
	t.Helper()
	res, err := cs.ListTools(context.Background(), nil)
	require.NoError(t, err)
	out := map[string]map[string]string{}
	for _, tl := range res.Tools {
		raw, err := json.Marshal(tl.InputSchema)
		require.NoError(t, err)
		var schema struct {
			Properties map[string]struct {
				Description string `json:"description"`
			} `json:"properties"`
		}
		require.NoError(t, json.Unmarshal(raw, &schema))
		out[tl.Name] = map[string]string{}
		for name, p := range schema.Properties {
			out[tl.Name][name] = p.Description
		}
	}
	return out
}

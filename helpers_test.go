package ahasendmcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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
	server, err := NewServer(&mcp.Implementation{Name: "ahasend-test", Version: "test"}, opts)
	require.NoError(t, err)

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
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

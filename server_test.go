package ahasendmcp

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServerRefusesInvalidOptions(t *testing.T) {
	f := newFakeAPI(t)
	impl := &mcp.Implementation{Name: "test", Version: "test"}
	tests := []struct {
		name   string
		change func(*Options)
	}{
		{"nil client", func(o *Options) { o.Client = nil }},
		{"zero account ID", func(o *Options) { o.AccountID = uuid.Nil }},
		{"empty dashboard base URL", func(o *Options) { o.DashboardBaseURL = "" }},
		{"31-byte confirmation key", func(o *Options) { o.ConfirmationKey = make([]byte, 31) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := testOptions(t, f)
			tt.change(&opts)
			server, err := NewServer(impl, opts)
			assert.Error(t, err)
			assert.Nil(t, server)
		})
	}

	t.Run("valid options", func(t *testing.T) {
		server, err := NewServer(impl, testOptions(t, f))
		require.NoError(t, err)
		assert.NotNil(t, server)
	})
}

func TestNilScopesListEveryTool(t *testing.T) {
	cs := connect(t, testOptions(t, newFakeAPI(t)))
	assert.ElementsMatch(t, toolNames(""), listedNames(t, cs))
}

func TestScopesHideAndShowTools(t *testing.T) {
	tests := []struct {
		name   string
		scopes []string
		want   []string
	}{
		{
			name:   "no scopes list no tools",
			scopes: []string{},
			want:   []string{},
		},
		{
			name:   "family met by :all",
			scopes: []string{"messages:read:all"},
			want:   []string{"list_messages", "get_message", "diagnose_message"},
		},
		{
			name:   "family met by a domain scope",
			scopes: []string{"messages:send:{mail.example.com}"},
			want:   []string{"send_email", "send_conversation_email", "send_template_email"},
		},
		{
			name:   "static scope met only by itself",
			scopes: []string{"domains:read", "domains:write:all"},
			want:   []string{"list_domains", "get_domain"},
		},
		{
			name:   "bare family name does not meet the family",
			scopes: []string{"messages:read", "domains:delete"},
			want:   []string{},
		},
		{
			name:   "domain delete through a domain scope",
			scopes: []string{"domains:delete:{mail.example.com}", "statistics-transactional:read:all"},
			want:   []string{"delete_domain", "get_deliverability_stats", "get_bounce_stats", "get_delivery_time_stats"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := testOptions(t, newFakeAPI(t))
			opts.Scopes = tt.scopes
			cs := connect(t, opts)
			assert.ElementsMatch(t, tt.want, listedNames(t, cs))
		})
	}
}

func TestHasScope(t *testing.T) {
	assert.True(t, hasScope([]string{"messages:read:all"}, "messages:read"))
	assert.True(t, hasScope([]string{"messages:read:{example.com}"}, "messages:read"))
	assert.False(t, hasScope([]string{"messages:read"}, "messages:read"))
	assert.False(t, hasScope([]string{"messages:readx:all"}, "messages:read"))
	assert.True(t, hasScope([]string{"domains:write"}, "domains:write"))
	assert.False(t, hasScope([]string{"domains:write:all"}, "domains:write"))
	assert.False(t, hasScope(nil, "domains:write"))
}

func TestToolsetsNarrowTheList(t *testing.T) {
	opts := testOptions(t, newFakeAPI(t))
	opts.Toolsets = []string{ToolsetSending, "no-such-toolset"}
	cs := connect(t, opts)
	assert.ElementsMatch(t, toolNames(ToolsetSending), listedNames(t, cs))
}

func TestReadOnlyNarrowsTheList(t *testing.T) {
	opts := testOptions(t, newFakeAPI(t))
	opts.ReadOnly = true
	cs := connect(t, opts)

	var want []string
	for _, info := range Catalogue() {
		if info.Annotations.ReadOnlyHint {
			want = append(want, info.Name)
		}
	}
	assert.ElementsMatch(t, want, listedNames(t, cs))
	assert.NotContains(t, listedNames(t, cs), "send_email")
}

func TestScopesToolsetsAndReadOnlyCombine(t *testing.T) {
	opts := testOptions(t, newFakeAPI(t))
	opts.Scopes = []string{"domains:read", "domains:write", "messages:read:all"}
	opts.Toolsets = []string{ToolsetDomains}
	opts.ReadOnly = true
	cs := connect(t, opts)
	assert.ElementsMatch(t, []string{"list_domains", "get_domain"}, listedNames(t, cs))
}

func TestToolsListIsCacheablePrivately(t *testing.T) {
	cs := connect(t, testOptions(t, newFakeAPI(t)))
	res, err := cs.ListTools(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, "private", res.CacheScope)
	assert.Equal(t, 300000, res.TTLMs)
}

func TestListedToolsCarryTitlesAnnotationsAndSchemas(t *testing.T) {
	cs := connect(t, testOptions(t, newFakeAPI(t)))
	res, err := cs.ListTools(context.Background(), nil)
	require.NoError(t, err)
	for _, tl := range res.Tools {
		assert.NotEmpty(t, tl.Title, tl.Name)
		assert.NotEmpty(t, tl.Description, tl.Name)
		require.NotNil(t, tl.Annotations, tl.Name)
		assert.NotNil(t, tl.Annotations.DestructiveHint, tl.Name)
		assert.NotNil(t, tl.Annotations.OpenWorldHint, tl.Name)
		assert.NotNil(t, tl.InputSchema, tl.Name)
		assert.NotNil(t, tl.OutputSchema, tl.Name)
	}
}

func TestUnlistedToolIsUnknown(t *testing.T) {
	opts := testOptions(t, newFakeAPI(t))
	opts.Scopes = []string{"domains:read"}
	cs := connect(t, opts)
	_, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "send_email",
		Arguments: map[string]any{},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestBlockedRefusesEveryListedTool(t *testing.T) {
	const blockText = "This account is suspended. Contact AhaSend support."
	f := newFakeAPI(t)
	rec := &fakeRecorder{}
	opts := testOptions(t, f)
	opts.Scopes = nil
	opts.Blocked = blockText
	opts.Recorder = rec
	cs := connect(t, opts)

	unblocked := testOptions(t, f)
	unblocked.Scopes = nil
	names := listedNames(t, cs)
	assert.ElementsMatch(t, listedNames(t, connect(t, unblocked)), names, "the tool list is unchanged")
	require.Len(t, names, len(Catalogue()))

	// Empty arguments: the block text wins over argument checks.
	for _, name := range names {
		res := callTool(t, cs, name, map[string]any{})
		assert.Equal(t, blockText, requireToolError(t, res), name)
	}
	assert.Empty(t, f.calls(), "no API call")

	rec.mu.Lock()
	for _, call := range rec.calls {
		assert.Equal(t, ErrorKindBlocked, call.ErrorKind, call.Tool)
		assert.False(t, call.Success)
	}
	assert.Len(t, rec.calls, len(names))
	rec.mu.Unlock()

	narrow := testOptions(t, f)
	narrow.Scopes = []string{"domains:read"}
	narrow.Blocked = blockText
	_, err := connect(t, narrow).CallTool(context.Background(), &mcp.CallToolParams{Name: "send_email", Arguments: map[string]any{}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}

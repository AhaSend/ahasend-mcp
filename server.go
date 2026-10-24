// Package ahasendmcp holds the AhaSend MCP tools. NewServer builds an MCP
// server whose tools call the AhaSend v2 API through ahasend-go.
package ahasendmcp

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// DefaultMaxRecipientsPerSend is the one-off send limit used when
// Options.MaxRecipientsPerSend is zero.
const DefaultMaxRecipientsPerSend = 50

// minConfirmationKeyLen is the shortest ConfirmationKey NewServer accepts.
const minConfirmationKeyLen = 32

// toolsListTTL is how long a client may cache the tool list.
const toolsListTTL = 5 * time.Minute

// schemaCache is shared by every server NewServer builds, so a server built
// per request does not derive the tool schemas again.
var schemaCache = mcp.NewSchemaCache()

// Options configures the server NewServer builds.
type Options struct {
	Client               *api.APIClient // ahasend-go client, with base URL and key
	AccountID            uuid.UUID
	Scopes               []string // the key's v2 scopes; nil: unknown, list every tool
	Toolsets             []string // nil: every toolset
	ReadOnly             bool
	DashboardBaseURL     string   // for the campaign confirmation link
	ConfirmationKey      []byte   // HMAC key for campaign confirmation, at least 32 bytes
	Stateless            bool     // true when served statelessly (hosted)
	MaxRecipientsPerSend int      // 0: 50
	ReadInbound          bool     // true: message tools also return received email
	Blocked              string   // non-empty: every listed tool's call returns this as an error
	Recorder             Recorder // nil: no recording
}

// NewServer returns an MCP server with the tools Options allows. It
// returns an error for a nil impl, a nil Client, a zero AccountID, an
// empty DashboardBaseURL, or a ConfirmationKey shorter than 32 bytes.
func NewServer(impl *mcp.Implementation, opts Options) (*mcp.Server, error) {
	if impl == nil {
		return nil, errors.New("ahasendmcp: nil Implementation")
	}
	if opts.Client == nil {
		return nil, errors.New("ahasendmcp: Options.Client is nil")
	}
	if opts.AccountID == uuid.Nil {
		return nil, errors.New("ahasendmcp: Options.AccountID is zero")
	}
	if opts.DashboardBaseURL == "" {
		return nil, errors.New("ahasendmcp: Options.DashboardBaseURL is empty")
	}
	if len(opts.ConfirmationKey) < minConfirmationKeyLen {
		return nil, errors.New("ahasendmcp: Options.ConfirmationKey is shorter than 32 bytes")
	}

	env := newToolEnv(opts)
	server := mcp.NewServer(impl, &mcp.ServerOptions{
		SchemaCache:  schemaCache,
		SetCacheable: setToolsListCacheable,
	})

	listed := make(map[string]ToolInfo)
	for _, t := range allTools() {
		if !visible(t.info, opts) {
			continue
		}
		t.add(server, t.mcpTool(env), env)
		listed[t.info.Name] = t.info
	}
	addResources(server)
	// Each middleware added wraps the ones before it, so the recorder sees
	// the blocked answer.
	if opts.Blocked != "" {
		server.AddReceivingMiddleware(blockedMiddleware(opts.Blocked, listed))
	}
	if opts.Recorder != nil {
		server.AddReceivingMiddleware(recordingMiddleware(opts.Recorder, listed))
	}
	return server, nil
}

// setToolsListCacheable lets a client cache the tool list for this
// connection only, for toolsListTTL.
func setToolsListCacheable(_ context.Context, req mcp.Request, c *mcp.Cacheable) {
	if _, ok := req.(*mcp.ListToolsRequest); !ok {
		return
	}
	c.CacheScope = "private"
	c.TTLMs = int(toolsListTTL / time.Millisecond)
}

// toolEnv is what every tool handler reads: the API client and the parts of
// Options that change tool behaviour.
type toolEnv struct {
	client           *api.APIClient
	accountID        uuid.UUID
	scopes           []string
	readInbound      bool
	maxRecipients    int
	dashboardBaseURL string
	confirmationKey  []byte
	stateless        bool
}

func newToolEnv(opts Options) *toolEnv {
	maxRecipients := opts.MaxRecipientsPerSend
	if maxRecipients <= 0 {
		maxRecipients = DefaultMaxRecipientsPerSend
	}
	return &toolEnv{
		client:           opts.Client,
		accountID:        opts.AccountID,
		scopes:           opts.Scopes,
		readInbound:      opts.ReadInbound,
		maxRecipients:    maxRecipients,
		dashboardBaseURL: strings.TrimRight(opts.DashboardBaseURL, "/"),
		confirmationKey:  bytes.Clone(opts.ConfirmationKey),
		stateless:        opts.Stateless,
	}
}

// mayUse reports whether the key's scopes meet the required scope, or the
// scopes are unknown.
func (e *toolEnv) mayUse(required string) bool {
	return e.scopes == nil || hasScope(e.scopes, required)
}

// blockedMiddleware answers every call of a listed tool with text as a tool
// error, before the SDK checks the arguments, so the agent learns why the
// connection cannot act whatever it sends. A tool that is not listed is
// left to the SDK, which answers it as unknown.
func blockedMiddleware(text string, listed map[string]ToolInfo) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			call, ok := req.(*mcp.CallToolRequest)
			if !ok || call.Params == nil {
				return next(ctx, method, req)
			}
			if _, ok := listed[call.Params.Name]; !ok {
				return next(ctx, method, req)
			}
			res := &mcp.CallToolResult{}
			res.SetError(&toolError{kind: ErrorKindBlocked, message: text})
			return res, nil
		}
	}
}

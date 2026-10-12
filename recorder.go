package ahasendmcp

import (
	"context"
	"errors"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Recorder receives one ToolCall for every call of a listed tool.
type Recorder interface {
	ToolCalled(ctx context.Context, call ToolCall)
}

// ToolCall describes one tool call. It never holds the call's arguments or
// result.
type ToolCall struct {
	Tool, Toolset string
	Success       bool
	ErrorKind     string // empty on success; otherwise one of the ErrorKind constants
	Duration      time.Duration
}

// recordingMiddleware reports every call of a listed tool to rec. A call of
// a tool that is not listed is answered by the SDK as an unknown tool and is
// not recorded.
func recordingMiddleware(rec Recorder, listed map[string]ToolInfo) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			call, ok := req.(*mcp.CallToolRequest)
			if !ok || call.Params == nil {
				return next(ctx, method, req)
			}
			info, ok := listed[call.Params.Name]
			if !ok {
				return next(ctx, method, req)
			}

			start := time.Now()
			res, err := next(ctx, method, req)
			kind := callErrorKind(res, err)
			rec.ToolCalled(ctx, ToolCall{
				Tool:      info.Name,
				Toolset:   info.Toolset,
				Success:   kind == "",
				ErrorKind: kind,
				Duration:  time.Since(start),
			})
			return res, err
		}
	}
}

// callErrorKind returns the ErrorKind of a finished tools/call request.
func callErrorKind(res mcp.Result, err error) string {
	if err != nil {
		// The handler failed outside its own error handling.
		return ErrorKindAPI5xx
	}
	result, ok := res.(*mcp.CallToolResult)
	if !ok || result == nil || !result.IsError {
		return ""
	}
	var te *toolError
	if errors.As(result.GetError(), &te) {
		return te.kind
	}
	// The SDK refused the arguments before the handler ran.
	return ErrorKindInvalidInput
}

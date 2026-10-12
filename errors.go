package ahasendmcp

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/AhaSend/ahasend-go/api"
)

// ToolCall.ErrorKind values.
const (
	// ErrorKindAPI4xx is a v2 API 4xx answer.
	ErrorKindAPI4xx = "api_4xx"
	// ErrorKindAPI5xx is a v2 API 5xx answer or a network failure.
	ErrorKindAPI5xx = "api_5xx"
	// ErrorKindInvalidInput is a call refused before any API call.
	ErrorKindInvalidInput = "invalid_input"
	// ErrorKindDeclined is a campaign send the user said no to.
	ErrorKindDeclined = "declined"
	// ErrorKindInboundRefused is a received message the tool refused.
	ErrorKindInboundRefused = "inbound_refused"
	// ErrorKindBlocked is a call refused because Options.Blocked is set.
	ErrorKindBlocked = "blocked"
)

// toolError is a tool failure returned to the agent as a tool result with
// isError set. kind is the ToolCall.ErrorKind it is recorded with.
type toolError struct {
	kind    string
	message string
}

func (e *toolError) Error() string { return e.message }

// invalidInput refuses a call before any API call.
func invalidInput(format string, args ...any) *toolError {
	return &toolError{kind: ErrorKindInvalidInput, message: fmt.Sprintf(format, args...)}
}

// toolErrorFrom turns an error from a tool handler into a toolError with a
// message that says what to do next.
func toolErrorFrom(err error) *toolError {
	var te *toolError
	if errors.As(err, &te) {
		return te
	}
	var apiErr *api.APIError
	if errors.As(err, &apiErr) {
		return apiErrorMessage(apiErr)
	}
	return serverFailure()
}

func serverFailure() *toolError {
	return &toolError{
		kind:    ErrorKindAPI5xx,
		message: "AhaSend could not handle the request. Retry later.",
	}
}

// apiErrorMessage maps a v2 API error to the tool error text.
func apiErrorMessage(e *api.APIError) *toolError {
	message := strings.TrimSpace(e.Message)
	if message == "" && e.StatusCode != 0 {
		message = http.StatusText(e.StatusCode)
	}

	switch {
	case e.StatusCode == 0:
		// ahasend-go refused the request before sending it.
		if e.Type == api.ErrorTypeValidation {
			return &toolError{kind: ErrorKindInvalidInput, message: message}
		}
		if e.Type == api.ErrorTypeRateLimit {
			// The client's own limit; no request reached the API.
			return &toolError{kind: ErrorKindInvalidInput, message: rateLimited().message}
		}
		if e.Type == api.ErrorTypeAuthentication {
			return &toolError{kind: ErrorKindInvalidInput, message: "No AhaSend API key is set for this connection."}
		}
		return serverFailure()
	case e.StatusCode == http.StatusUnauthorized:
		return &toolError{
			kind: ErrorKindAPI4xx,
			message: "AhaSend refused this connection's credentials: the account may be paused, " +
				"or the connection's API key deleted. Check the AhaSend dashboard.",
		}
	case e.StatusCode == http.StatusForbidden:
		return &toolError{
			kind: ErrorKindAPI4xx,
			message: fmt.Sprintf("AhaSend refused the request: %s. This connection's scopes can be "+
				"changed on the API Keys page of the AhaSend dashboard, or by connecting again.", strings.TrimRight(message, ". ")),
		}
	case e.StatusCode == http.StatusNotFound:
		return &toolError{kind: ErrorKindAPI4xx, message: "Not found in this AhaSend account."}
	case e.StatusCode == http.StatusTooManyRequests:
		return rateLimited()
	case e.StatusCode >= 500:
		return serverFailure()
	default:
		// 400, 409, 422 and every other 4xx: the API's own message.
		return &toolError{
			kind:    ErrorKindAPI4xx,
			message: fmt.Sprintf("AhaSend refused the request (HTTP %d): %s", e.StatusCode, message),
		}
	}
}

func rateLimited() *toolError {
	return &toolError{
		kind:    ErrorKindAPI4xx,
		message: "AhaSend is rate limiting requests. Wait a moment, then retry.",
	}
}

// isForbidden reports whether err is a v2 API 403 answer.
func isForbidden(err error) bool {
	var apiErr *api.APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusForbidden
}

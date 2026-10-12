package ahasendmcp

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/google/uuid"
)

// ToolCall.ErrorKind values.
const (
	// ErrorKindAPI4xx is a v2 API 4xx answer.
	ErrorKindAPI4xx = "api_4xx"
	// ErrorKindAPI5xx is a v2 API 5xx answer or a network failure.
	ErrorKindAPI5xx = "api_5xx"
	// ErrorKindInvalidInput is a call the module refused before making the
	// API call it was asked for, such as a send over the one-off send limit or
	// a campaign send for a campaign that is not a draft.
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

// parseID reads a required ID argument, refusing the call when it is not an
// ID.
func parseID(field, value string) (uuid.UUID, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return uuid.Nil, invalidInput("%s is required.", field)
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, invalidInput("%s must be an ID, such as 6b1e1f43-5d2c-4b0e-9a57-0c1f2a3b4c5d.", field)
	}
	return id, nil
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
		if message == "" || strings.EqualFold(message, http.StatusText(http.StatusNotFound)) {
			return &toolError{kind: ErrorKindAPI4xx, message: "Not found in this AhaSend account."}
		}
		// The API's message names what was not found, such as the list a
		// request named.
		return &toolError{kind: ErrorKindAPI4xx, message: "Not found in this AhaSend account: " + strings.TrimRight(message, ". ") + "."}
	case e.StatusCode == http.StatusTooManyRequests:
		if message != "" && !strings.EqualFold(message, http.StatusText(http.StatusTooManyRequests)) {
			// A limit with its own reason, such as a daily cap, may not
			// lift in a moment, so the API's words are kept.
			text := "AhaSend is limiting requests: " + strings.TrimRight(message, ". ") + ". Wait before you retry"
			if strings.Contains(strings.ToLower(message), "daily") {
				text += "; a daily limit lifts only the next day"
			}
			return &toolError{kind: ErrorKindAPI4xx, message: text + "."}
		}
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

// toolErrorFor maps err as toolErrorFrom does. A bare 404 (no message, or
// only "Not Found") from a marketing tool is the answer of an account
// without marketing, since a missing item has its own message, so it says
// so.
func toolErrorFor(info ToolInfo, err error) error {
	te := toolErrorFrom(err)
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound || !isMarketingTool(info) {
		return te
	}
	message := strings.TrimSpace(apiErr.Message)
	if message != "" && !strings.EqualFold(message, http.StatusText(http.StatusNotFound)) {
		return te
	}
	return &toolError{kind: te.kind, message: te.message + " Contacts, lists, campaigns and campaign templates " +
		"exist only on accounts with marketing turned on."}
}

// isMarketingTool reports whether a tool works on contacts, lists, campaigns
// or campaign templates.
func isMarketingTool(info ToolInfo) bool {
	for _, scope := range info.RequiredScopes {
		for _, prefix := range []string{"contacts:", "lists:", "campaigns:"} {
			if strings.HasPrefix(scope, prefix) {
				return true
			}
		}
	}
	return false
}

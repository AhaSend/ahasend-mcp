package ahasendmcp

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendEmail(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("POST "+apiPath("/messages"), http.StatusOK, sendResponse("a@example.org", "b@example.org"))
	cs := connect(t, testOptions(t, f))

	res := callTool(t, cs, "send_email", map[string]any{
		"from":           map[string]any{"email": "app@mail.example.com", "name": "App"},
		"recipients":     []map[string]any{{"email": "a@example.org", "substitutions": map[string]any{"first": "A"}}, {"email": "b@example.org"}},
		"subject":        "Hello {{ first }}",
		"html_content":   "<p>Hi</p>",
		"sandbox":        true,
		"sandbox_result": "deliver",
		"tags":           []string{"welcome"},
	})
	out := decode[sendOutput](t, res)

	req := f.onlyCall(t)
	assert.Equal(t, http.MethodPost, req.Method)
	assert.Equal(t, apiPath("/messages"), req.Path)
	assert.Equal(t, map[string]any{"email": "app@mail.example.com", "name": "App"}, req.Body["from"])
	assert.Equal(t, "Hello {{ first }}", req.Body["subject"])
	assert.Equal(t, "<p>Hi</p>", req.Body["html_content"])
	assert.Equal(t, true, req.Body["sandbox"])
	assert.Equal(t, "deliver", req.Body["sandbox_result"])
	assert.NotContains(t, req.Body, "text_content")
	recipients := req.Body["recipients"].([]any)
	require.Len(t, recipients, 2)
	assert.Equal(t, map[string]any{"email": "a@example.org", "substitutions": map[string]any{"first": "A"}}, recipients[0])

	require.Len(t, out.Messages, 2)
	assert.Equal(t, "a@example.org", out.Messages[0].Recipient)
	assert.Equal(t, "queued", out.Messages[0].Status)
	assert.NotEmpty(t, out.Messages[0].ID)
}

func TestSendEmailAttachments(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("POST "+apiPath("/messages"), http.StatusOK, sendResponse("a@example.org"))
	cs := connect(t, testOptions(t, f))

	callTool(t, cs, "send_email", map[string]any{
		"from":         map[string]any{"email": "app@mail.example.com"},
		"recipients":   []map[string]any{{"email": "a@example.org"}},
		"subject":      "Invoice",
		"html_content": `<img src="cid:logo@example.com">`,
		"attachments": []map[string]any{
			{"file_name": "invoice.pdf", "content_type": "application/pdf", "data": "JVBERg==", "base64": true},
			{"file_name": "logo.png", "content_type": "image/png", "data": "iVBORw==", "base64": true, "content_id": "logo@example.com"},
		},
	})

	attachments := f.onlyCall(t).Body["attachments"].([]any)
	require.Len(t, attachments, 2)
	file := attachments[0].(map[string]any)
	assert.Equal(t, "invoice.pdf", file["file_name"])
	assert.Equal(t, "application/pdf", file["content_type"])
	assert.Empty(t, file["content_disposition"], "the API writes the disposition with the file name")
	assert.NotContains(t, file, "content_id")
	inline := attachments[1].(map[string]any)
	assert.Equal(t, "<logo@example.com>", inline["content_id"])
	assert.Empty(t, inline["content_disposition"])
}

func TestContentID(t *testing.T) {
	assert.Equal(t, "", contentID(""))
	assert.Equal(t, "<logo@example.com>", contentID("logo@example.com"))
	assert.Equal(t, "<logo@example.com>", contentID(" <logo@example.com> "))
}

func TestSendConversationEmail(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("POST "+apiPath("/messages/conversation"), http.StatusOK, sendResponse("to@example.org"))
	cs := connect(t, testOptions(t, f))

	res := callTool(t, cs, "send_conversation_email", map[string]any{
		"from":         map[string]any{"email": "me@mail.example.com"},
		"to":           []map[string]any{{"email": "to@example.org"}},
		"cc":           []map[string]any{{"email": "cc@example.org", "name": "CC"}},
		"subject":      "Re: question",
		"text_content": "Answer",
		"reply_to":     map[string]any{"email": "support@mail.example.com"},
	})
	out := decode[sendOutput](t, res)

	req := f.onlyCall(t)
	assert.Equal(t, http.MethodPost, req.Method)
	assert.Equal(t, apiPath("/messages/conversation"), req.Path)
	assert.Equal(t, []any{map[string]any{"email": "to@example.org"}}, req.Body["to"])
	assert.Equal(t, []any{map[string]any{"email": "cc@example.org", "name": "CC"}}, req.Body["cc"])
	assert.NotContains(t, req.Body, "bcc")
	assert.Equal(t, "Answer", req.Body["text_content"])
	assert.Equal(t, map[string]any{"email": "support@mail.example.com"}, req.Body["reply_to"])
	assert.NotContains(t, req.Body, "sandbox")
	require.Len(t, out.Messages, 1)
}

func TestSendTemplateEmail(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("POST "+apiPath("/messages/template"), http.StatusOK, sendResponse("a@example.org"))
	cs := connect(t, testOptions(t, f))

	const templateID = "5a4b3c2d-1e0f-4a9b-8c7d-6e5f4a3b2c1d"
	res := callTool(t, cs, "send_template_email", map[string]any{
		"template_id": templateID,
		"recipients":  []map[string]any{{"email": "a@example.org", "substitutions": map[string]any{"code": "1234"}}},
		"send_at":     "2026-10-13T09:00:00Z",
	})
	out := decode[sendOutput](t, res)

	req := f.onlyCall(t)
	assert.Equal(t, http.MethodPost, req.Method)
	assert.Equal(t, apiPath("/messages/template"), req.Path)
	assert.Equal(t, templateID, req.Body["template_id"])
	assert.NotContains(t, req.Body, "from")
	assert.NotContains(t, req.Body, "subject")
	assert.Equal(t, map[string]any{"first_attempt": "2026-10-13T09:00:00Z"}, req.Body["schedule"])
	require.Len(t, out.Messages, 1)
}

func TestSendTemplateEmailRefusesABadTemplateID(t *testing.T) {
	f := newFakeAPI(t)
	cs := connect(t, testOptions(t, f))
	res := callTool(t, cs, "send_template_email", map[string]any{
		"template_id": "welcome",
		"recipients":  []map[string]any{{"email": "a@example.org"}},
	})
	requireToolError(t, res)
	assert.Empty(t, f.calls())
}

func TestCancelScheduledEmail(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("DELETE "+apiPath("/messages/{id}/cancel"), http.StatusOK, map[string]any{"message": "Message cancelled"})
	cs := connect(t, testOptions(t, f))

	out := decode[confirmationOutput](t, callTool(t, cs, "cancel_scheduled_email", map[string]any{"message_id": outboundMessageID}))

	req := f.onlyCall(t)
	assert.Equal(t, http.MethodDelete, req.Method)
	assert.Equal(t, apiPath("/messages/"+outboundMessageID+"/cancel"), req.Path)
	assert.Equal(t, "Message cancelled", out.Message)
}

func recipientList(n int) []map[string]any {
	out := make([]map[string]any, n)
	for i := range out {
		out[i] = map[string]any{"email": fmt.Sprintf("r%d@example.org", i)}
	}
	return out
}

func TestOneOffSendLimit(t *testing.T) {
	tests := []struct {
		tool string
		path string
		args func(n int) map[string]any
	}{
		{
			tool: "send_email",
			path: "/messages",
			args: func(n int) map[string]any {
				return map[string]any{
					"from":         map[string]any{"email": "app@mail.example.com"},
					"recipients":   recipientList(n),
					"subject":      "Hi",
					"text_content": "Hi",
				}
			},
		},
		{
			tool: "send_template_email",
			path: "/messages/template",
			args: func(n int) map[string]any {
				return map[string]any{
					"template_id": "5a4b3c2d-1e0f-4a9b-8c7d-6e5f4a3b2c1d",
					"recipients":  recipientList(n),
				}
			},
		},
		{
			tool: "send_conversation_email",
			path: "/messages/conversation",
			args: func(n int) map[string]any {
				// n addresses across to, cc and bcc.
				all := recipientList(n)
				return map[string]any{
					"from":         map[string]any{"email": "app@mail.example.com"},
					"to":           all[:n/3],
					"cc":           all[n/3 : 2*n/3],
					"bcc":          all[2*n/3:],
					"subject":      "Hi",
					"text_content": "Hi",
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.tool+" refuses 51", func(t *testing.T) {
			f := newFakeAPI(t)
			rec := &fakeRecorder{}
			opts := testOptions(t, f)
			opts.Recorder = rec
			cs := connect(t, opts)

			text := requireToolError(t, callTool(t, cs, tt.tool, tt.args(51)))
			assert.Contains(t, text, "51 recipients")
			assert.Contains(t, text, "campaign")
			assert.Empty(t, f.calls(), "no API call")
			assert.Equal(t, ErrorKindInvalidInput, rec.only(t).ErrorKind)
		})
		t.Run(tt.tool+" passes 50", func(t *testing.T) {
			f := newFakeAPI(t)
			f.handle("POST "+apiPath(tt.path), http.StatusOK, sendResponse("r0@example.org"))
			cs := connect(t, testOptions(t, f))

			res := callTool(t, cs, tt.tool, tt.args(50))
			assert.False(t, res.IsError, resultText(res))
			assert.Len(t, f.calls(), 1)
		})
	}
}

func TestOneOffSendLimitFollowsOptions(t *testing.T) {
	f := newFakeAPI(t)
	opts := testOptions(t, f)
	opts.MaxRecipientsPerSend = 2
	cs := connect(t, opts)
	text := requireToolError(t, callTool(t, cs, "send_email", map[string]any{
		"from":         map[string]any{"email": "app@mail.example.com"},
		"recipients":   recipientList(3),
		"subject":      "Hi",
		"text_content": "Hi",
	}))
	assert.Contains(t, text, "at most 2")
	assert.Empty(t, f.calls())
}

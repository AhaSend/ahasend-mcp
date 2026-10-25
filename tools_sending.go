package ahasendmcp

import (
	"context"
	"strings"
	"time"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
)

func sendingTools() []tool {
	return []tool{
		newTool(ToolInfo{
			Name:           "send_email",
			Title:          "Send email",
			Toolset:        ToolsetSending,
			RequiredScopes: []string{"messages:send"},
			Annotations:    annWriteOpenWorld(),
		}, "Sends one email, with content given in the call, to each recipient in recipients; every recipient "+
			"gets a separate message and sees only their own address. Use it for transactional email such as "+
			"receipts, notifications and tests, not for bulk or marketing email, which belongs in a campaign. "+
			"The sender must be on a domain of this account with valid DNS records. Set sandbox to true to "+
			"test without delivering; sandbox_result then picks the outcome. The result lists one entry per "+
			"recipient with the new message ID and its status, or an error for that recipient.",
			sendEmail),
		newTool(ToolInfo{
			Name:           "send_conversation_email",
			Title:          "Send conversation email",
			Toolset:        ToolsetSending,
			RequiredScopes: []string{"messages:send"},
			Annotations:    annWriteOpenWorld(),
		}, "Sends one email to to, cc and bcc recipients together, as a person writes an email: every to and cc "+
			"recipient sees the others. Use it for a reply or a conversation, and send_email for separate "+
			"transactional messages. The sender must be on a domain of this account with valid DNS records. "+
			"Set sandbox to true to test without delivering. The result lists one entry per recipient with the "+
			"message ID and its status, or an error for that recipient.",
			sendConversationEmail),
		newTool(ToolInfo{
			Name:           "send_template_email",
			Title:          "Send template email",
			Toolset:        ToolsetSending,
			RequiredScopes: []string{"messages:send"},
			Annotations:    annWriteOpenWorld(),
		}, "Sends a stored transactional template to each recipient in recipients; every recipient gets a "+
			"separate message. Use it instead of send_email when the content is a template in this account. "+
			"Each recipient's substitutions fill the template's variables: get_template lists them, and each "+
			"one's kind tells what to send, with a list taking an array of objects with its item_fields as keys. "+
			"Without from or subject, the template's default sender and subject are used. Set sandbox to true to test without "+
			"delivering. The result lists one entry per recipient with the new message ID and its status, or "+
			"an error for that recipient.",
			sendTemplateEmail),
		newTool(ToolInfo{
			Name:           "cancel_scheduled_email",
			Title:          "Cancel scheduled email",
			Toolset:        ToolsetSending,
			RequiredScopes: []string{"messages:cancel"},
			Annotations:    annDestructive(),
		}, "Cancels a message that was scheduled for later delivery and has not been sent yet. Use it with the "+
			"message ID a send returned. AhaSend confirms the request even for a message that was already sent, "+
			"so the confirmation does not prove the message was stopped; get_message shows its status.",
			cancelScheduledEmail),
	}
}

// addressInput is an email address with an optional display name.
type addressInput struct {
	Email string `json:"email" jsonschema:"Email address."`
	Name  string `json:"name,omitempty" jsonschema:"Display name."`
}

func (a addressInput) sender() common.SenderAddress {
	s := common.SenderAddress{Email: a.Email}
	if a.Name != "" {
		s.Name = ptr(a.Name)
	}
	return s
}

func senderPtr(a *addressInput) *common.SenderAddress {
	if a == nil {
		return nil
	}
	s := a.sender()
	return &s
}

func senders(in []addressInput) []common.SenderAddress {
	if len(in) == 0 {
		return nil
	}
	out := make([]common.SenderAddress, 0, len(in))
	for _, a := range in {
		out = append(out, a.sender())
	}
	return out
}

// recipientInput is one recipient of a send that gives each recipient a
// separate message.
type recipientInput struct {
	Email         string         `json:"email" jsonschema:"Recipient email address."`
	Name          string         `json:"name,omitempty" jsonschema:"Recipient display name."`
	Substitutions map[string]any `json:"substitutions,omitempty" jsonschema:"This recipient's values for the variables in the subject and content. For a template, each variable's kind tells what to send, and a list takes an array of objects with its item_fields as keys."`
}

func recipients(in []recipientInput) []common.Recipient {
	out := make([]common.Recipient, 0, len(in))
	for _, r := range in {
		rec := common.Recipient{Email: r.Email, Substitutions: r.Substitutions}
		if r.Name != "" {
			rec.Name = ptr(r.Name)
		}
		out = append(out, rec)
	}
	return out
}

// attachmentInput is one file attached to a message.
type attachmentInput struct {
	FileName    string `json:"file_name" jsonschema:"File name shown to the recipient."`
	ContentType string `json:"content_type" jsonschema:"MIME type, such as application/pdf."`
	Data        string `json:"data" jsonschema:"File content: plain UTF-8 text, or base64 when base64 is true."`
	Base64      bool   `json:"base64,omitempty" jsonschema:"True when data is base64 encoded."`
	ContentID   string `json:"content_id,omitempty" jsonschema:"Content-ID that makes this file an inline image, such as logo@example.com. Reference it from the HTML as cid:logo@example.com, without angle brackets."`
}

func attachments(in []attachmentInput) []common.Attachment {
	if len(in) == 0 {
		return nil
	}
	out := make([]common.Attachment, 0, len(in))
	for _, a := range in {
		// No content disposition: the API then writes it with the file
		// name, which it leaves out when one is given.
		att := common.Attachment{
			FileName:    a.FileName,
			ContentType: a.ContentType,
			Data:        a.Data,
			Base64:      a.Base64,
		}
		if id := contentID(a.ContentID); id != "" {
			att.ContentID = ptr(id)
		}
		out = append(out, att)
	}
	return out
}

// sendOptions are the send settings every send tool takes.
type sendOptions struct {
	ReplyTo       *addressInput     `json:"reply_to,omitempty" jsonschema:"Reply-to address."`
	Headers       map[string]string `json:"headers,omitempty" jsonschema:"Extra email headers by name."`
	Tags          []string          `json:"tags,omitempty" jsonschema:"Tags to find the messages by later."`
	Attachments   []attachmentInput `json:"attachments,omitempty" jsonschema:"Files to attach."`
	Sandbox       bool              `json:"sandbox,omitempty" jsonschema:"True to test the send without delivering it."`
	SandboxResult string            `json:"sandbox_result,omitempty" jsonschema:"With sandbox: the outcome to simulate, one of deliver, bounce, defer, fail, suppress."`
	TrackOpens    *bool             `json:"track_opens,omitempty" jsonschema:"Track opens; omit to use the account setting."`
	TrackClicks   *bool             `json:"track_clicks,omitempty" jsonschema:"Track clicks; omit to use the account setting."`
	SendAt        *time.Time        `json:"send_at,omitempty" jsonschema:"RFC 3339 time for the first delivery attempt, within 7 days; omit to send now."`
	ExpiresAt     *time.Time        `json:"expires_at,omitempty" jsonschema:"RFC 3339 time after which an undelivered message is dropped, within 8 days."`
}

func (o sendOptions) sandbox() *bool {
	if !o.Sandbox {
		return nil
	}
	return ptr(true)
}

func (o sendOptions) sandboxResult() *string {
	if o.SandboxResult == "" {
		return nil
	}
	return ptr(o.SandboxResult)
}

func (o sendOptions) tracking() *common.Tracking {
	if o.TrackOpens == nil && o.TrackClicks == nil {
		return nil
	}
	return &common.Tracking{Open: o.TrackOpens, Click: o.TrackClicks}
}

func (o sendOptions) schedule() *common.MessageSchedule {
	if o.SendAt == nil && o.ExpiresAt == nil {
		return nil
	}
	return &common.MessageSchedule{FirstAttempt: o.SendAt, Expires: o.ExpiresAt}
}

type sendEmailInput struct {
	From          addressInput     `json:"from" jsonschema:"Sender, on a domain of this account."`
	Recipients    []recipientInput `json:"recipients" jsonschema:"Recipients; each gets a separate message."`
	Subject       string           `json:"subject" jsonschema:"Subject line."`
	TextContent   string           `json:"text_content,omitempty" jsonschema:"Plain text body. Give text_content, html_content or both."`
	HTMLContent   string           `json:"html_content,omitempty" jsonschema:"HTML body. Give text_content, html_content or both."`
	Substitutions map[string]any   `json:"substitutions,omitempty" jsonschema:"Values for every recipient's variables; a recipient's own substitutions replace them."`
	sendOptions
}

type sendConversationInput struct {
	From        addressInput   `json:"from" jsonschema:"Sender, on a domain of this account."`
	To          []addressInput `json:"to" jsonschema:"To recipients; at least one."`
	CC          []addressInput `json:"cc,omitempty" jsonschema:"Cc recipients."`
	BCC         []addressInput `json:"bcc,omitempty" jsonschema:"Bcc recipients."`
	Subject     string         `json:"subject" jsonschema:"Subject line."`
	TextContent string         `json:"text_content,omitempty" jsonschema:"Plain text body. Give text_content, html_content or both."`
	HTMLContent string         `json:"html_content,omitempty" jsonschema:"HTML body. Give text_content, html_content or both."`
	sendOptions
}

type sendTemplateInput struct {
	TemplateID string           `json:"template_id" jsonschema:"ID of a transactional template of this account."`
	From       *addressInput    `json:"from,omitempty" jsonschema:"Sender; omit to use the template's default sender."`
	Recipients []recipientInput `json:"recipients" jsonschema:"Recipients; each gets a separate message filled with its own substitutions."`
	Subject    string           `json:"subject,omitempty" jsonschema:"Subject line; omit to use the template's subject."`
	sendOptions
}

type sendOutput struct {
	Messages []sentMessage `json:"messages" jsonschema:"One entry per recipient."`
}

type sentMessage struct {
	ID        string     `json:"id,omitempty" jsonschema:"Message ID, for get_message, diagnose_message and cancel_scheduled_email."`
	Recipient string     `json:"recipient" jsonschema:"Recipient email address."`
	Status    string     `json:"status" jsonschema:"Status of the message right after the send."`
	Error     string     `json:"error,omitempty" jsonschema:"Why this recipient's message was not sent."`
	SendAt    *time.Time `json:"send_at,omitempty" jsonschema:"Scheduled first delivery attempt."`
}

func sendOutputFrom(resp *responses.CreateMessageResponse) sendOutput {
	out := sendOutput{Messages: []sentMessage{}}
	for _, m := range resp.Data {
		sent := sentMessage{Recipient: m.Recipient.Email, Status: m.Status}
		if m.ID != nil {
			sent.ID = *m.ID
		}
		if m.Error != nil {
			sent.Error = *m.Error
		}
		if m.Schedule != nil {
			sent.SendAt = m.Schedule.FirstAttempt
		}
		out.Messages = append(out.Messages, sent)
	}
	return out
}

// checkRecipientCount applies the one-off send limit.
func checkRecipientCount(env *toolEnv, count int) error {
	if count > env.maxRecipients {
		return invalidInput("This send has %d recipients, and one send may have at most %d. "+
			"For bulk email, use a campaign.", count, env.maxRecipients)
	}
	return nil
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return ptr(s)
}

func sendEmail(ctx context.Context, env *toolEnv, in sendEmailInput) (sendOutput, error) {
	if err := checkRecipientCount(env, len(in.Recipients)); err != nil {
		return sendOutput{}, err
	}
	req := requests.CreateMessageRequest{
		From:          in.From.sender(),
		Recipients:    recipients(in.Recipients),
		Subject:       in.Subject,
		ReplyTo:       senderPtr(in.ReplyTo),
		TextContent:   optionalString(in.TextContent),
		HtmlContent:   optionalString(in.HTMLContent),
		Attachments:   attachments(in.Attachments),
		Headers:       in.Headers,
		Substitutions: in.Substitutions,
		Tags:          in.Tags,
		Sandbox:       in.sandbox(),
		SandboxResult: in.sandboxResult(),
		Tracking:      in.tracking(),
		Schedule:      in.schedule(),
	}
	resp, _, err := env.client.MessagesAPI.CreateMessage(ctx, env.accountID, req)
	if err != nil {
		return sendOutput{}, err
	}
	return sendOutputFrom(resp), nil
}

func sendConversationEmail(ctx context.Context, env *toolEnv, in sendConversationInput) (sendOutput, error) {
	if err := checkRecipientCount(env, len(in.To)+len(in.CC)+len(in.BCC)); err != nil {
		return sendOutput{}, err
	}
	req := requests.CreateConversationMessageRequest{
		From:          in.From.sender(),
		To:            senders(in.To),
		CC:            senders(in.CC),
		BCC:           senders(in.BCC),
		Subject:       in.Subject,
		ReplyTo:       senderPtr(in.ReplyTo),
		TextContent:   optionalString(in.TextContent),
		HtmlContent:   optionalString(in.HTMLContent),
		Attachments:   attachments(in.Attachments),
		Headers:       in.Headers,
		Tags:          in.Tags,
		Sandbox:       in.sandbox(),
		SandboxResult: in.sandboxResult(),
		Tracking:      in.tracking(),
		Schedule:      in.schedule(),
	}
	resp, _, err := env.client.MessagesAPI.CreateConversationMessage(ctx, env.accountID, req)
	if err != nil {
		return sendOutput{}, err
	}
	return sendOutputFrom(resp), nil
}

func sendTemplateEmail(ctx context.Context, env *toolEnv, in sendTemplateInput) (sendOutput, error) {
	if err := checkRecipientCount(env, len(in.Recipients)); err != nil {
		return sendOutput{}, err
	}
	templateID, err := uuid.Parse(strings.TrimSpace(in.TemplateID))
	if err != nil {
		return sendOutput{}, invalidInput("template_id must be the ID of a transactional template.")
	}
	req := requests.CreateTemplateMessageRequest{
		TemplateID:    templateID,
		From:          senderPtr(in.From),
		Recipients:    recipients(in.Recipients),
		ReplyTo:       senderPtr(in.ReplyTo),
		Subject:       in.Subject,
		Attachments:   attachments(in.Attachments),
		Headers:       in.Headers,
		Tags:          in.Tags,
		Sandbox:       in.sandbox(),
		SandboxResult: in.sandboxResult(),
		Tracking:      in.tracking(),
		Schedule:      in.schedule(),
	}
	resp, _, err := env.client.MessagesAPI.CreateTemplateMessage(ctx, env.accountID, req)
	if err != nil {
		return sendOutput{}, err
	}
	return sendOutputFrom(resp), nil
}

type messageIDInput struct {
	MessageID string `json:"message_id" jsonschema:"Message ID, as a send tool or list_messages returned it."`
}

type confirmationOutput struct {
	Message string `json:"message" jsonschema:"AhaSend's confirmation."`
}

func cancelScheduledEmail(ctx context.Context, env *toolEnv, in messageIDInput) (confirmationOutput, error) {
	id := strings.TrimSpace(in.MessageID)
	if id == "" {
		return confirmationOutput{}, invalidInput("message_id is required.")
	}
	resp, _, err := env.client.MessagesAPI.CancelMessage(ctx, env.accountID, id)
	if err != nil {
		return confirmationOutput{}, err
	}
	return confirmationOutput{Message: resp.Message}, nil
}

// contentID returns id as a MIME Content-ID, in angle brackets, which the
// API needs to send the file inline.
func contentID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" || strings.HasPrefix(id, "<") {
		return id
	}
	return "<" + id + ">"
}

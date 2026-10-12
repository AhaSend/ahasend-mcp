package ahasendmcp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The campaign send tools.
const (
	toolSendCampaign     = "send_campaign"
	toolScheduleCampaign = "schedule_campaign"
)

const (
	// confirmInputID names the confirmation form among a call's input
	// requests, and confirmField is the form's one yes/no field.
	confirmInputID = "confirm_send"
	confirmField   = "send"
	// confirmationTTL is how long the user has to answer the question.
	confirmationTTL = 10 * time.Minute
	// multiRoundTripVersion is the first protocol version whose clients
	// answer input requests themselves, so a stateless server can ask them.
	multiRoundTripVersion = "2026-07-28"
)

type sendCampaignInput struct {
	CampaignID string `json:"campaign_id" jsonschema:"ID of the draft campaign to send now."`
}

type scheduleCampaignInput struct {
	CampaignID string    `json:"campaign_id" jsonschema:"ID of the draft campaign to schedule."`
	SendAt     time.Time `json:"send_at" jsonschema:"RFC 3339 time to send at, at least 1 minute ahead."`
}

type campaignSendOutput struct {
	Sent         bool       `json:"sent" jsonschema:"True when AhaSend accepted the send or schedule. False when nothing was sent."`
	Message      string     `json:"message" jsonschema:"What happened, and what to do next."`
	Status       string     `json:"status,omitempty" jsonschema:"The campaign's status after the send was accepted, normally validating."`
	SendAt       *time.Time `json:"send_at,omitempty" jsonschema:"The scheduled send time, for schedule_campaign."`
	Warnings     []string   `json:"warnings,omitempty" jsonschema:"Things the send accepted but the user should know."`
	DashboardURL string     `json:"dashboard_url,omitempty" jsonschema:"The campaign's page in the AhaSend dashboard, where the user can send it, when nothing was sent."`
}

// minScheduleLead is how far ahead the API needs a send time to be.
const minScheduleLead = time.Minute

// newCampaignSendTool builds send_campaign, or schedule_campaign when
// scheduled is true. Both always ask the user before they send.
func newCampaignSendTool(info ToolInfo, description string, scheduled bool) tool {
	if scheduled {
		return campaignSendTool(info, description, func(in scheduleCampaignInput) (string, *time.Time, error) {
			if in.SendAt.IsZero() {
				return "", nil, invalidInput("send_at is required.")
			}
			sendAt := in.SendAt.UTC()
			if sendAt.Before(time.Now().Add(minScheduleLead)) {
				return "", nil, invalidInput("send_at must be at least a minute from now; use send_campaign to send now.")
			}
			return in.CampaignID, &sendAt, nil
		})
	}
	return campaignSendTool(info, description, func(in sendCampaignInput) (string, *time.Time, error) {
		return in.CampaignID, nil, nil
	})
}

func campaignSendTool[In any](info ToolInfo, description string, read func(In) (string, *time.Time, error)) tool {
	return tool{
		info:        info,
		description: description,
		add: func(s *mcp.Server, t *mcp.Tool, env *toolEnv) {
			mcp.AddTool(s, t, func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, campaignSendOutput, error) {
				rawID, sendAt, err := read(in)
				if err != nil {
					return nil, campaignSendOutput{}, toolErrorFrom(err)
				}
				res, out, err := confirmCampaignSend(ctx, env, req, info.Name, rawID, sendAt)
				if err != nil {
					return nil, campaignSendOutput{}, toolErrorFor(info, err)
				}
				return res, out, nil
			})
		},
	}
}

// confirmCampaignSend sends a campaign only on the user's yes to a question
// this server asked, carried back with the question's signed state, and only
// when the campaign has not changed since. A call without a valid answer
// asks the question, or, when the client cannot be asked, sends nothing and
// returns the campaign's dashboard URL.
func confirmCampaignSend(ctx context.Context, env *toolEnv, req *mcp.CallToolRequest, toolName, rawID string, sendAt *time.Time) (*mcp.CallToolResult, campaignSendOutput, error) {
	campaignID, err := parseID("campaign_id", rawID)
	if err != nil {
		return nil, campaignSendOutput{}, err
	}
	call := confirmCall{tool: toolName, accountID: env.accountID, campaignID: campaignID, sendAt: sendAt}

	answered := req.Params != nil && len(req.Params.InputResponses) > 0
	if answered {
		state, ok := openState(env.confirmationKey, req.Params.RequestState, time.Now())
		if ok && state.isFor(call) {
			if !acceptedYes(req.Params.InputResponses[confirmInputID]) {
				return nil, campaignSendOutput{}, &toolError{
					kind:    ErrorKindDeclined,
					message: "The user did not confirm the " + call.action() + ", so nothing was sent.",
				}
			}
			campaign, _, err := env.client.CampaignsAPI.GetCampaign(ctx, env.accountID, campaignID)
			if err != nil {
				return readFailure(env, call, err)
			}
			if state.matches(campaign) {
				out, err := sendCampaign(ctx, env, call)
				return nil, out, err
			}
		}
		if !answersInputRequests(req) {
			// The SDK asked an older client for this call and cannot ask it
			// again within the same call.
			return nil, campaignSendOutput{
				Message: "Nothing was sent: the answer did not match the question (the campaign changed, or " +
					"the answer was late or not valid). Call " + toolName + " again to ask the user about the " +
					"campaign as it is now.",
			}, nil
		}
	}

	if !canAsk(env, req) {
		return nil, dashboardOutput(env, call,
			"Nothing was sent: this client cannot ask the user to confirm the "+call.action()+"."), nil
	}
	return askToConfirm(ctx, env, call)
}

// confirmCall is what a campaign send call asks for.
type confirmCall struct {
	tool       string
	accountID  uuid.UUID
	campaignID uuid.UUID
	sendAt     *time.Time
}

func (c confirmCall) action() string {
	if c.sendAt != nil {
		return "schedule"
	}
	return "send"
}

func (c confirmCall) sendAtText() string {
	if c.sendAt == nil {
		return ""
	}
	return c.sendAt.UTC().Format(time.RFC3339Nano)
}

// answersInputRequests reports whether the client answers input requests
// itself (protocol 2026-07-28 or later). For an older client the SDK asks it
// through the session and calls the tool again with the answer.
func answersInputRequests(req *mcp.CallToolRequest) bool {
	return req.ProtocolVersion() >= multiRoundTripVersion
}

// canAsk reports whether the client can show the confirmation form: it has
// form elicitation, as the SDK counts it, and the server can reach it, which
// a stateless server can only on 2026-07-28 or later.
func canAsk(env *toolEnv, req *mcp.CallToolRequest) bool {
	caps := req.ClientCapabilities()
	if caps == nil || caps.Elicitation == nil {
		return false
	}
	// An elicitation capability that names neither form nor url means form.
	if caps.Elicitation.Form == nil && caps.Elicitation.URL != nil {
		return false
	}
	return answersInputRequests(req) || !env.stateless
}

// acceptedYes reports whether the user accepted the form with yes.
func acceptedYes(resp mcp.InputResponse) bool {
	result, ok := resp.(*mcp.ElicitResult)
	if !ok || result == nil || result.Action != "accept" {
		return false
	}
	yes, ok := result.Content[confirmField].(bool)
	return ok && yes
}

// readFailure answers a failed read of the campaign or a list. With unknown
// scopes, a 403 means the key may not read what the question shows, so the
// user is sent to the dashboard instead.
func readFailure(env *toolEnv, call confirmCall, err error) (*mcp.CallToolResult, campaignSendOutput, error) {
	if env.scopes == nil && isForbidden(err) {
		return nil, dashboardOutput(env, call, "Nothing was sent: this connection may not read the campaign "+
			"and its lists, which the confirmation shows the user."), nil
	}
	return nil, campaignSendOutput{}, err
}

func dashboardOutput(env *toolEnv, call confirmCall, reason string) campaignSendOutput {
	url := fmt.Sprintf("%s/account/%s/marketing/campaigns/%s", env.dashboardBaseURL, call.accountID, call.campaignID)
	return campaignSendOutput{
		Message:      reason + " The user can " + call.action() + " the campaign in the AhaSend dashboard at dashboard_url.",
		DashboardURL: url,
	}
}

func sendCampaign(ctx context.Context, env *toolEnv, call confirmCall) (campaignSendOutput, error) {
	resp, _, err := env.client.CampaignsAPI.SendCampaign(ctx, env.accountID, call.campaignID,
		requests.SendCampaignRequest{SendAt: call.sendAt})
	if err != nil {
		return campaignSendOutput{}, campaignNotReadyError(err)
	}
	out := campaignSendOutput{
		Sent:     true,
		Status:   resp.Status,
		SendAt:   call.sendAt,
		Warnings: resp.Warnings,
		Message: "AhaSend accepted the " + call.action() + ". The campaign's lists are checked, and it may be " +
			"reviewed, before mail goes out; get_campaign shows its status.",
	}
	return out, nil
}

// askToConfirm reads the campaign and its lists and returns the question as
// an input request, with its signed state.
func askToConfirm(ctx context.Context, env *toolEnv, call confirmCall) (*mcp.CallToolResult, campaignSendOutput, error) {
	campaign, _, err := env.client.CampaignsAPI.GetCampaign(ctx, env.accountID, call.campaignID)
	if err != nil {
		return readFailure(env, call, err)
	}
	if campaign.Status != common.CampaignStatusDraft {
		// Only a draft can be sent or scheduled.
		return nil, campaignSendOutput{}, invalidInput("The campaign's status is %s, and only a draft can be sent or "+
			"scheduled, so the user was not asked. A scheduled campaign must be unscheduled first.", campaign.Status)
	}
	if campaign.Ready != nil && !*campaign.Ready {
		// Asking would waste the user's yes on a send the API refuses.
		return nil, campaignSendOutput{}, campaignIssuesError(campaign.Issues)
	}
	lists := make([]*responses.ContactList, 0, len(campaign.ListIDs))
	for _, id := range campaign.ListIDs {
		list, _, err := env.client.ListsAPI.GetList(ctx, env.accountID, id)
		if err != nil {
			return readFailure(env, call, err)
		}
		lists = append(lists, list)
	}
	if totalContacts(lists) == 0 {
		// The send would be refused: there is no one to send to.
		return nil, campaignSendOutput{}, invalidInput("The campaign's lists have no reachable contacts (0 in total), " +
			"so the user was not asked and nothing was sent. Add confirmed, enabled contacts to the lists first.")
	}

	state := newConfirmState(call, campaign, time.Now().Add(confirmationTTL))
	signed, err := signState(env.confirmationKey, state)
	if err != nil {
		return nil, campaignSendOutput{}, err
	}
	return &mcp.CallToolResult{
		InputRequests: mcp.InputRequestMap{
			confirmInputID: &mcp.ElicitParams{
				Mode:            "form",
				Message:         confirmationMessage(call, campaign, lists),
				RequestedSchema: confirmationSchema(call),
			},
		},
		RequestState: signed,
	}, campaignSendOutput{}, nil
}

// confirmationMessage is the question the user reads. Some clients show only
// its first lines, so the first line gives the send time and the contact
// count, and the next lines the campaign's name and subject. The facts
// AhaSend derives come first; every value the agent can write, such as names
// and the subject, is shown through userText, so it cannot add lines that
// look like part of the summary.
func confirmationMessage(call confirmCall, c *responses.Campaign, lists []*responses.ContactList) string {
	var b strings.Builder
	listWord := "lists"
	if len(lists) == 1 {
		listWord = "list"
	}
	reach := fmt.Sprintf("%d contacts on %d %s", totalContacts(lists), len(lists), listWord)
	if call.sendAt != nil {
		fmt.Fprintf(&b, "Schedule this campaign for %s, to %s?\n", sendTimeText(*call.sendAt), reach)
	} else {
		fmt.Fprintf(&b, "Send this campaign now to %s?\n", reach)
	}
	fmt.Fprintf(&b, "Campaign: %s\n", userText(c.Name))
	subject := "(none)"
	if s := deref(c.Subject); s != "" {
		subject = userText(s)
	}
	fmt.Fprintf(&b, "Subject: %s\n", subject)
	sender := "(none)"
	if c.From != nil && c.From.Email != "" {
		sender = userText(campaignSenderText(c.From))
	}
	fmt.Fprintf(&b, "Sender: %s\n", sender)
	b.WriteString("Lists:\n")
	if len(lists) == 0 {
		b.WriteString("- (none)\n")
	}
	for _, l := range lists {
		fmt.Fprintf(&b, "- %s: %d contacts\n", userText(l.Name), l.ContactCount)
	}
	b.WriteString("\nA contact on several lists gets the campaign once. Contact counts can change before the send.")
	return b.String()
}

// totalContacts adds up the lists' contact counts.
func totalContacts(lists []*responses.ContactList) int {
	total := 0
	for _, l := range lists {
		total += l.ContactCount
	}
	return total
}

// sendTimeText formats a send time in UTC, with seconds only when they are
// not zero.
func sendTimeText(t time.Time) string {
	t = t.UTC()
	if t.Second() != 0 || t.Nanosecond() != 0 {
		return t.Format("Mon, 02 Jan 2006 15:04:05 MST")
	}
	return t.Format("Mon, 02 Jan 2006 15:04 MST")
}

// userTextLimit is the most characters userText shows of one value.
const userTextLimit = 100

// userText shows a value the agent could have written on one line, in double
// quotes: control characters, line and paragraph separators and bidi
// controls become spaces, runs of spaces become one, double quotes become
// single quotes, and the value is cut to userTextLimit characters.
func userText(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case unicode.IsControl(r),
			r == '\u2028', r == '\u2029',
			r >= '\u202a' && r <= '\u202e',
			r >= '\u2066' && r <= '\u2069',
			r == '\u200e', r == '\u200f', r == '\u061c':
			return ' '
		case r == '"':
			return '\''
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if runes := []rune(s); len(runes) > userTextLimit {
		s = string(runes[:userTextLimit]) + "…"
	}
	return `"` + s + `"`
}

func confirmationSchema(call confirmCall) map[string]any {
	title := "Send the campaign"
	if call.sendAt != nil {
		title = "Schedule the campaign"
	}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			confirmField: map[string]any{
				"type":        "boolean",
				"title":       title,
				"description": "Yes to " + call.action() + " the campaign as shown, no to cancel.",
			},
		},
		"required": []string{confirmField},
	}
}

// confirmState is what the question was about. It travels to the client and
// back as the signed requestState.
type confirmState struct {
	Tool        string   `json:"tool"`
	AccountID   string   `json:"account_id"`
	CampaignID  string   `json:"campaign_id"`
	SendAt      string   `json:"send_at"`
	UpdatedAt   string   `json:"updated_at"`
	ListIDs     []string `json:"list_ids"`
	ContentHash string   `json:"content_hash"`
	ExpiresAt   int64    `json:"expires_at"`
}

func newConfirmState(call confirmCall, c *responses.Campaign, expires time.Time) confirmState {
	return confirmState{
		Tool:        call.tool,
		AccountID:   call.accountID.String(),
		CampaignID:  call.campaignID.String(),
		SendAt:      call.sendAtText(),
		UpdatedAt:   c.UpdatedAt.UTC().Format(time.RFC3339Nano),
		ListIDs:     campaignListIDs(c),
		ContentHash: contentHash(c.Content),
		ExpiresAt:   expires.Unix(),
	}
}

// isFor reports whether the state is for this call: the same tool, account,
// campaign and send time.
func (s *confirmState) isFor(call confirmCall) bool {
	return s.Tool == call.tool &&
		s.AccountID == call.accountID.String() &&
		s.CampaignID == call.campaignID.String() &&
		s.SendAt == call.sendAtText()
}

// matches reports whether the campaign is still the one the user was asked
// about: the same updated_at, lists and content.
func (s *confirmState) matches(c *responses.Campaign) bool {
	return s.UpdatedAt == c.UpdatedAt.UTC().Format(time.RFC3339Nano) &&
		slices.Equal(s.ListIDs, campaignListIDs(c)) &&
		s.ContentHash == contentHash(c.Content)
}

func campaignListIDs(c *responses.Campaign) []string {
	ids := make([]string, 0, len(c.ListIDs))
	for _, id := range c.ListIDs {
		ids = append(ids, id.String())
	}
	return ids
}

// contentHash is the SHA-256 of a campaign's content.
func contentHash(content *responses.TemplateContent) string {
	raw, err := json.Marshal(content)
	if err != nil {
		// TemplateContent holds only strings and a bool, so this cannot
		// happen; an empty hash never equals a real one.
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// signState encodes the state as base64url JSON and appends its
// HMAC-SHA256 under key.
func signState(key []byte, s confirmState) (string, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(raw)
	return payload + "." + base64.RawURLEncoding.EncodeToString(stateMAC(key, payload)), nil
}

// openState returns the state when its signature is valid and it has not
// expired at now.
func openState(key []byte, signed string, now time.Time) (*confirmState, bool) {
	payload, sig, ok := strings.Cut(signed, ".")
	if !ok {
		return nil, false
	}
	got, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(got, stateMAC(key, payload)) {
		return nil, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, false
	}
	var s confirmState
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, false
	}
	if !now.Before(time.Unix(s.ExpiresAt, 0)) {
		return nil, false
	}
	return &s, true
}

func stateMAC(key []byte, payload string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	return mac.Sum(nil)
}

// campaignIssuesError refuses a campaign that is not ready to send, with
// the issues to fix first.
func campaignIssuesError(issues []responses.CampaignIssue) error {
	var b strings.Builder
	b.WriteString("The campaign is not ready to send, so the user was not asked. Fix these first:")
	for _, issue := range issues {
		fmt.Fprintf(&b, " %s (%s).", strings.TrimRight(issue.Message, ". "), issue.Code)
	}
	return &toolError{kind: ErrorKindInvalidInput, message: b.String()}
}

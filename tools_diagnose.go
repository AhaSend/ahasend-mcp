package ahasendmcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
)

// Scopes of the optional diagnose_message checks.
const (
	scopeSuppressionsRead = "suppressions:read"
	scopeDomainsRead      = "domains:read"
)

// recentMessagesLimit is how many of a recipient's latest messages
// diagnose_message uses, and recipientPages how many pages of the
// recipient search it reads to find them.
const (
	recentMessagesLimit = 20
	recipientPages      = 3
)

type diagnoseInput struct {
	MessageID string     `json:"message_id,omitempty" jsonschema:"ID of the sent message to diagnose. Give message_id or recipient."`
	Recipient string     `json:"recipient,omitempty" jsonschema:"Recipient email address; the latest email sent to it is diagnosed. Give message_id or recipient."`
	FromTime  *time.Time `json:"from_time,omitempty" jsonschema:"With recipient: RFC 3339 start of the window to look in. Not used with message_id."`
	ToTime    *time.Time `json:"to_time,omitempty" jsonschema:"With recipient: RFC 3339 end of the window to look in. Not used with message_id."`
}

type diagnoseOutput struct {
	Found          bool              `json:"found" jsonschema:"False when no sent email matched; then only likely_cause and suggested_fix are set."`
	Message        *diagnosedMessage `json:"message,omitempty" jsonschema:"The message diagnosed."`
	RecentMessages []recentMessage   `json:"recent_messages,omitempty" jsonschema:"With recipient: the other recent email sent to the recipient, newest first."`
	Suppressions   []suppressionInfo `json:"suppressions,omitempty" jsonschema:"Suppressions of the recipient. Missing when the check found none or was skipped."`
	Domain         *domainState      `json:"domain,omitempty" jsonschema:"DNS state of the sender domain."`
	Skipped        []skippedCheck    `json:"skipped,omitempty" jsonschema:"Checks that were not made, with the scope each needs."`
	LikelyCause    string            `json:"likely_cause" jsonschema:"The most likely explanation."`
	SuggestedFix   string            `json:"suggested_fix" jsonschema:"What to do about it."`
}

type diagnosedMessage struct {
	ID                   string            `json:"id" jsonschema:"Message ID."`
	Sender               string            `json:"sender" jsonschema:"Sender email address."`
	Recipient            string            `json:"recipient" jsonschema:"Recipient email address."`
	Subject              string            `json:"subject" jsonschema:"Subject line."`
	Status               string            `json:"status" jsonschema:"Delivery status."`
	CreatedAt            time.Time         `json:"created_at" jsonschema:"When AhaSend accepted the message."`
	DeliveredAt          *time.Time        `json:"delivered_at,omitempty" jsonschema:"When the receiving server accepted the message."`
	BounceClassification string            `json:"bounce_classification,omitempty" jsonschema:"Bounce category, when the message bounced."`
	BounceReason         string            `json:"bounce_reason,omitempty" jsonschema:"The last failed attempt's answer from the receiving server."`
	DeliveryAttempts     []deliveryAttempt `json:"delivery_attempts" jsonschema:"Delivery attempts, oldest first."`
}

type recentMessage struct {
	ID        string    `json:"id" jsonschema:"Message ID."`
	Status    string    `json:"status" jsonschema:"Delivery status."`
	CreatedAt time.Time `json:"created_at" jsonschema:"When AhaSend accepted the message."`
}

type suppressionInfo struct {
	Email     string    `json:"email" jsonschema:"Suppressed address."`
	Domain    string    `json:"domain,omitempty" jsonschema:"Sender domain the suppression applies to; empty for every domain."`
	Reason    string    `json:"reason,omitempty" jsonschema:"Why the address is suppressed."`
	CreatedAt time.Time `json:"created_at" jsonschema:"When the suppression was made."`
	ExpiresAt time.Time `json:"expires_at" jsonschema:"When the suppression ends."`
}

type domainState struct {
	Domain         string      `json:"domain" jsonschema:"Sender domain."`
	Exists         bool        `json:"exists" jsonschema:"False when the domain is not in this account."`
	DNSValid       bool        `json:"dns_valid" jsonschema:"True when every required DNS record is in place."`
	Paused         bool        `json:"paused" jsonschema:"True when sending from the domain is paused."`
	PauseReason    string      `json:"pause_reason,omitempty" jsonschema:"Why sending is paused."`
	LastDNSCheckAt *time.Time  `json:"last_dns_check_at,omitempty" jsonschema:"When the DNS records were last checked."`
	MissingRecords []dnsRecord `json:"missing_records,omitempty" jsonschema:"Required DNS records that are not in place."`
}

type skippedCheck struct {
	Check  string `json:"check" jsonschema:"The check: suppressions or domain."`
	Scope  string `json:"scope,omitempty" jsonschema:"The scope the check needs, when a missing scope stopped it."`
	Reason string `json:"reason" jsonschema:"Why it was skipped."`
}

var errMissingTarget = invalidInput("Give message_id or recipient, not both and not neither.")

func diagnoseMessage(ctx context.Context, env *toolEnv, in diagnoseInput) (diagnoseOutput, error) {
	messageID := strings.TrimSpace(in.MessageID)
	recipient := strings.TrimSpace(in.Recipient)
	if (messageID == "") == (recipient == "") {
		return diagnoseOutput{}, errMissingTarget
	}

	var out diagnoseOutput
	var msg *responses.Message
	if messageID != "" {
		m, _, err := env.client.MessagesAPI.GetMessageByAPIID(ctx, env.accountID, messageID)
		if err != nil {
			return diagnoseOutput{}, err
		}
		if isReceived(m) {
			return diagnoseOutput{}, &toolError{
				kind:    ErrorKindInboundRefused,
				message: "This message is received email; diagnose_message diagnoses only email the account sent.",
			}
		}
		msg = m
	} else {
		latest, others, searchCut, err := latestSentTo(ctx, env, recipient, in.FromTime, in.ToTime)
		if err != nil {
			return diagnoseOutput{}, err
		}
		if latest == nil && searchCut {
			return diagnoseOutput{
				Found:        false,
				LikelyCause:  "No email sent to this recipient was found among the most recent messages to addresses that contain it; the search stopped before the end.",
				SuggestedFix: "Give a narrower time window with from_time and to_time, or give the message_id.",
			}, nil
		}
		if latest == nil {
			return diagnoseOutput{
				Found:        false,
				LikelyCause:  "No email sent to this recipient was found.",
				SuggestedFix: "Check the address for typos, give a wider time window if one was set, or check that the email was sent from this account.",
			}, nil
		}
		msg = latest
		out.RecentMessages = others
	}

	out.Found = true
	out.Message = diagnosedMessageFrom(msg)

	suppressions, skipped, err := checkSuppressions(ctx, env, msg.Recipient)
	if err != nil {
		return diagnoseOutput{}, err
	}
	out.Suppressions = suppressions
	if skipped != nil {
		out.Skipped = append(out.Skipped, *skipped)
	}

	domain, skipped, err := checkDomain(ctx, env, senderDomain(msg.Sender))
	if err != nil {
		return diagnoseOutput{}, err
	}
	out.Domain = domain
	if skipped != nil {
		out.Skipped = append(out.Skipped, *skipped)
	}

	out.LikelyCause, out.SuggestedFix = likelyCause(out)
	return out, nil
}

// latestSentTo returns the newest outbound message to recipient and a
// summary of the other recent outbound ones. Received email is never used.
//
// The API's recipient filter matches any address that contains the text,
// so only messages whose recipient equals the address are kept, and up to
// recipientPages pages are read to find them. searchCut reports that none
// was found and more pages were left unread.
func latestSentTo(ctx context.Context, env *toolEnv, recipient string, from, to *time.Time) (latest *responses.Message, others []recentMessage, searchCut bool, err error) {
	recipient = strings.ToLower(recipient)
	params := requests.GetMessagesParams{
		Recipient: ptr(recipient),
		FromTime:  from,
		ToTime:    to,
		PaginationParams: common.PaginationParams{
			Limit: ptr(int32(maxListLimit)),
		},
	}
	var sent []*responses.Message
	more := false
	for page := 0; page < recipientPages && len(sent) < recentMessagesLimit; page++ {
		resp, _, err := env.client.MessagesAPI.GetMessages(ctx, env.accountID, params)
		if err != nil {
			return nil, nil, false, err
		}
		for i := range resp.Data {
			m := &resp.Data[i]
			if !isReceived(m) && strings.EqualFold(m.Recipient, recipient) && len(sent) < recentMessagesLimit {
				sent = append(sent, m)
			}
		}
		more = resp.Pagination.HasMore && resp.Pagination.NextCursor != nil
		if !more {
			break
		}
		params.After = resp.Pagination.NextCursor
	}
	if len(sent) == 0 {
		return nil, nil, more, nil
	}
	latest = sent[0]
	for _, m := range sent[1:] {
		if m.CreatedAt.After(latest.CreatedAt) {
			latest = m
		}
	}
	for _, m := range sent {
		if m != latest {
			others = append(others, recentMessage{ID: m.ID.String(), Status: m.Status, CreatedAt: m.CreatedAt})
		}
	}
	return latest, others, false, nil
}

func diagnosedMessageFrom(m *responses.Message) *diagnosedMessage {
	d := &diagnosedMessage{
		ID:               m.ID.String(),
		Sender:           m.Sender,
		Recipient:        m.Recipient,
		Subject:          m.Subject,
		Status:           m.Status,
		CreatedAt:        m.CreatedAt,
		DeliveredAt:      m.DeliveredAt,
		DeliveryAttempts: deliveryAttemptsFrom(m.DeliveryAttempts),
	}
	if m.BounceClassification != nil {
		d.BounceClassification = *m.BounceClassification
	}
	switch baseStatus(m.Status) {
	case "Bounced", "OOB", "Failed", "Deferred", "Administratively failed":
		if n := len(m.DeliveryAttempts); n > 0 {
			d.BounceReason = m.DeliveryAttempts[n-1].Log
		}
	}
	return d
}

// checkSuppressions reads the recipient's suppressions, or reports the check
// as skipped when the connection may not read them.
func checkSuppressions(ctx context.Context, env *toolEnv, email string) ([]suppressionInfo, *skippedCheck, error) {
	if !env.mayUse(scopeSuppressionsRead) {
		return nil, missingScope("suppressions", scopeSuppressionsRead), nil
	}
	params := requests.GetSuppressionsParams{
		Email:            ptr(email),
		PaginationParams: common.PaginationParams{Limit: ptr(int32(maxListLimit))},
	}
	resp, _, err := env.client.SuppressionsAPI.GetSuppressions(ctx, env.accountID, params)
	if err != nil {
		if isForbidden(err) {
			return nil, refusedCheck("suppressions", scopeSuppressionsRead, err), nil
		}
		return nil, failedCheck("suppressions", err), nil
	}
	out := []suppressionInfo{}
	now := time.Now()
	for _, s := range resp.Data {
		if !s.ExpiresAt.After(now) {
			continue // expired; it no longer applies
		}
		out = append(out, suppressionInfo{
			Email:     s.Email,
			Domain:    s.Domain,
			Reason:    s.Reason,
			CreatedAt: s.CreatedAt,
			ExpiresAt: s.ExpiresAt,
		})
	}
	return out, nil, nil
}

// checkDomain reads the sender domain's DNS state, or reports the check as
// skipped when the connection may not read domains.
func checkDomain(ctx context.Context, env *toolEnv, name string) (*domainState, *skippedCheck, error) {
	if name == "" {
		return nil, nil, nil
	}
	if !env.mayUse(scopeDomainsRead) {
		return nil, missingScope("domain", scopeDomainsRead), nil
	}
	d, _, err := env.client.DomainsAPI.GetDomain(ctx, env.accountID, name)
	if err != nil {
		var apiErr *api.APIError
		switch {
		case isForbidden(err):
			return nil, refusedCheck("domain", scopeDomainsRead, err), nil
		case errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound:
			return &domainState{Domain: name, Exists: false}, nil, nil
		}
		return nil, failedCheck("domain", err), nil
	}
	state := &domainState{
		Domain:         d.Domain,
		Exists:         true,
		DNSValid:       d.DNSValid,
		Paused:         d.Paused,
		LastDNSCheckAt: d.LastDNSCheckAt,
	}
	if d.PauseReason != nil {
		state.PauseReason = *d.PauseReason
	}
	for _, r := range d.DNSRecords {
		if r.Required && !r.Propagated {
			state.MissingRecords = append(state.MissingRecords, dnsRecordFrom(r))
		}
	}
	return state, nil, nil
}

func missingScope(check, scope string) *skippedCheck {
	return &skippedCheck{Check: check, Scope: scope, Reason: "This connection does not have the " + scope + " scope."}
}

// failedCheck lists a check that failed for another reason as skipped, so
// the rest of the diagnosis is still returned.
func failedCheck(check string, err error) *skippedCheck {
	return &skippedCheck{Check: check, Reason: "The check failed: " + toolErrorFrom(err).Error()}
}

func refusedCheck(check, scope string, err error) *skippedCheck {
	reason := "AhaSend refused the check"
	var apiErr *api.APIError
	if errors.As(err, &apiErr) && apiErr.Message != "" {
		reason += ": " + apiErr.Message
	}
	return &skippedCheck{Check: check, Scope: scope, Reason: reason + "."}
}

// senderDomain returns the domain part of an email address, lowercased.
func senderDomain(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSuffix(email[at+1:], ">"))
}

// baseStatus drops the Sandbox prefix of a sandbox send's status.
func baseStatus(status string) string {
	return strings.TrimPrefix(status, "Sandbox ")
}

// likelyCause explains a diagnosis in one cause and one fix.
func likelyCause(d diagnoseOutput) (cause, fix string) {
	m := d.Message
	status := baseStatus(m.Status)
	sandbox := status != m.Status

	switch {
	case status == "Suppressed":
		cause = "AhaSend did not send the message because the recipient is suppressed."
		fix = "Check the suppression's reason. Remove the suppression only when the recipient wants this email again."
	case d.Domain != nil && !d.Domain.Exists && !finished(status):
		cause = fmt.Sprintf("The sender domain %s is not in this account.", d.Domain.Domain)
		fix = "Add the domain, publish its DNS records, and send from it again."
	case d.Domain != nil && d.Domain.Paused && !finished(status):
		cause = fmt.Sprintf("Sending from %s is paused", d.Domain.Domain)
		if d.Domain.PauseReason != "" {
			cause += " (" + d.Domain.PauseReason + ")"
		}
		cause += "."
		fix = "Find and stop the cause of the pause, such as sending to invalid addresses, then ask AhaSend support (or, for a sub-account, its parent account) to resume the domain."
	case d.Domain != nil && !d.Domain.DNSValid && !finished(status):
		cause = fmt.Sprintf("The DNS records of %s are not all in place.", d.Domain.Domain)
		fix = "Add the records listed under missing_records at the DNS provider, then run check_domain_dns."
		if len(d.Domain.MissingRecords) == 0 {
			fix = "Every required record is in place, so check the domain's DMARC record and its alignment, then run check_domain_dns."
		}
	case status == "Bounced" || status == "OOB":
		cause = "The receiving server rejected the message"
		if m.BounceClassification != "" {
			cause += " (" + m.BounceClassification + ")"
		}
		cause += "."
		fix = "Read bounce_reason. Check the recipient address; when the reason names a block or a spam policy, review the content and the sending domain's reputation."
	case status == "Failed":
		cause = "AhaSend could not deliver the message."
		fix = "Read bounce_reason and the delivery attempts for the receiving server's answer."
	case status == "Administratively failed":
		cause = "AhaSend stopped the message before delivery."
		fix = "Read bounce_reason. When it does not explain the stop, contact AhaSend support with the message ID."
	case status == "Deferred":
		cause = "The receiving server deferred the message for now; AhaSend retries it automatically."
		fix = "Wait for the next attempts. When they keep failing, read bounce_reason for the receiving server's answer."
	case status == "Delivered" || status == "Opened" || status == "Clicked":
		cause = "The receiving server accepted the message."
		fix = "When the recipient cannot find it, ask them to check spam and filters, and check the sending domain's SPF, DKIM and DMARC records."
	default:
		cause = fmt.Sprintf("The message is not delivered yet (status %s).", m.Status)
		fix = "Wait, then check again."
	}

	if sandbox {
		cause = "This was a sandbox send, so nothing was delivered. " + cause
	}
	if status != "Suppressed" && suppressedFor(d.Suppressions, senderDomain(m.Sender)) {
		cause += " The recipient is suppressed now, so new email to them from this domain is not sent."
	}
	return cause, fix
}

// finished reports whether the receiving server has given its final answer,
// so the domain's DNS state today does not explain the outcome.
func finished(status string) bool {
	switch status {
	case "Delivered", "Opened", "Clicked", "Bounced", "OOB", "Failed", "Administratively failed":
		return true
	}
	return false
}

// suppressedFor reports whether a suppression applies to mail from domain:
// one for every domain, or one for that domain.
func suppressedFor(suppressions []suppressionInfo, domain string) bool {
	for _, s := range suppressions {
		if s.Domain == "" || strings.EqualFold(s.Domain, domain) {
			return true
		}
	}
	return false
}

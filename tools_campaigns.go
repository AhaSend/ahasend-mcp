package ahasendmcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
)

// campaignAudienceNote states the audience rules of the campaign write tools.
const campaignAudienceNote = "The audience is list_ids and segment_ids, less exclude_list_ids and " +
	"exclude_segment_ids. A campaign needs at least one list or segment to send to. A list or segment cannot be " +
	"both included and excluded. An exclusion wins: a contact with any membership of an excluded list, whatever " +
	"its status, or a member of an excluded segment gets nothing, also when an included list or segment reaches " +
	"it. A contact reached through several lists and segments gets one message. " + segmentUnsubscribeNote + " " +
	"While a segment of the audience is being rebuilt the write is refused with 409 and nothing changes: call " +
	"again in a minute. A segment must be ready before the campaign can be sent."

func campaignsTools() []tool {
	return []tool{
		newTool(ToolInfo{
			Name:           "list_campaigns",
			Title:          "List campaigns",
			Toolset:        ToolsetCampaigns,
			RequiredScopes: []string{"campaigns:read"},
			Annotations:    annRead(),
		}, "Lists the marketing campaigns of this account, newest first, with their status, subject, sender and "+
			"send times, but not their audience, content or issues. Use it to find a campaign; get_campaign returns "+
			"one in full. The result holds one page and a next_cursor for the page after it.",
			listCampaigns),
		newTool(ToolInfo{
			Name:           "get_campaign",
			Title:          "Get campaign",
			Toolset:        ToolsetCampaigns,
			RequiredScopes: []string{"campaigns:read"},
			Annotations:    annRead(),
		}, "Returns one campaign with its audience (lists, segments and exclusions), content and status. For a "+
			"draft, ready tells whether it can be sent and issues lists what to fix first, such as a missing "+
			"subject, sender, list or segment, or unsubscribe link. Use it to check a campaign before sending it. "+ahamlReadNote,
			getCampaign),
		newTool(ToolInfo{
			Name:           "create_campaign",
			Title:          "Create campaign",
			Toolset:        ToolsetCampaigns,
			RequiredScopes: []string{"campaigns:write"},
			Annotations:    annWrite(),
		}, "Creates a draft marketing campaign. Only name is required; the rest can be set later with "+
			"update_campaign. The design comes from markup, mjml or html (with optional text), from editor alone "+
			"(an empty design), or from a campaign template named by template_id, which the campaign copies. The "+
			"HTML must link to {{ unsubscribe_url }} or {{ unsubscribe_link }} before the campaign can be sent; "+
			`in AhaML, write <Link href="{{ unsubscribe_url }}">Unsubscribe</Link>. `+
			"Nothing is sent. "+campaignAudienceNote+" The result is the campaign, without its content, with ready "+
			"and the issues left to fix. "+ahamlWriteNote,
			createCampaign),
		newTool(ToolInfo{
			Name:           "update_campaign",
			Title:          "Update campaign",
			Toolset:        ToolsetCampaigns,
			RequiredScopes: []string{"campaigns:write"},
			Annotations:    annWrite(),
		}, "Changes a draft campaign; a campaign that is not a draft cannot be changed. Only the fields given "+
			"change. Each of list_ids, segment_ids, exclude_list_ids and exclude_segment_ids replaces its own set "+
			"when given, and [] empties it; to move a campaign from lists to segments, give segment_ids and an "+
			"empty list_ids. The design's editor cannot change: give markup for a Simple Editor design, mjml for "+
			"an advanced design and html for an html design. "+campaignAudienceNote+" The result is the campaign, "+
			"without its content, with ready and the issues left to fix. "+ahamlUpdateNote,
			updateCampaign),
		newTool(ToolInfo{
			Name:           "delete_campaign",
			Title:          "Delete campaign",
			Toolset:        ToolsetCampaigns,
			RequiredScopes: []string{"campaigns:delete"},
			Annotations:    annDestructive(),
		}, "Deletes a draft campaign with its design; a campaign that is not a draft cannot be deleted. Use it "+
			"only when the user asks to remove the campaign. The result is AhaSend's confirmation.",
			deleteCampaign),
		newTool(ToolInfo{
			Name:           "duplicate_campaign",
			Title:          "Duplicate campaign",
			Toolset:        ToolsetCampaigns,
			RequiredScopes: []string{"campaigns:write"},
			Annotations:    annWrite(),
		}, "Makes a new draft from a campaign in any status, with its design, subject, preview text, sender "+
			"and the lists and segments that still exist, included and excluded. Use it to send a campaign again "+
			"or start from an old one. While a segment of the audience is being rebuilt it is refused with 409: "+
			"call again in a minute. The result is the new draft, without its content.",
			duplicateCampaign),
		newTool(ToolInfo{
			Name:           "send_campaign_test",
			Title:          "Send campaign test",
			Toolset:        ToolsetCampaigns,
			RequiredScopes: []string{"campaigns:send"},
			Annotations:    annWriteOpenWorld(),
		}, "Sends a test of a draft campaign to 1 to 5 addresses, with placeholders for the contact's fields. "+
			"Use it so the user can check the email before the real send. A campaign with any issue but a "+
			"missing unsubscribe link is refused. An account can send 50 test emails a day (UTC), counted per "+
			"address. The result lists the addresses the mail server accepted and refused; when "+
			"result_unknown is true the test may have gone out, so check the inboxes before sending another.",
			sendCampaignTest),
		newCampaignSendTool(ToolInfo{
			Name:           toolSendCampaign,
			Title:          "Send campaign",
			Toolset:        ToolsetCampaigns,
			RequiredScopes: []string{"campaigns:send", "campaigns:read", "lists:read"},
			Annotations:    annWriteOpenWorld(),
		}, "Sends a draft campaign to its lists and segments now, after the user confirms. The user is always "+
			"asked first, in a form that shows the campaign, its lists and segments with their contact counts, "+
			"its exclusions, and the send time; "+
			"nothing is sent without their yes. When this client cannot show the form, nothing is sent and the "+
			"result holds dashboard_url, where the user sends the campaign. A campaign with issues, or with a "+
			"segment that is not ready, is refused; get_campaign and get_segment show them. After the send, the "+
			"campaign's contacts are checked and it may be reviewed "+
			"before mail goes out; get_campaign shows its status. The result says whether the send was accepted.",
			false),
		newCampaignSendTool(ToolInfo{
			Name:           toolScheduleCampaign,
			Title:          "Schedule campaign",
			Toolset:        ToolsetCampaigns,
			RequiredScopes: []string{"campaigns:send", "campaigns:read", "lists:read"},
			Annotations:    annWriteOpenWorld(),
		}, "Schedules a draft campaign to send at send_at, at least 1 minute ahead, after the user confirms. "+
			"The user is always asked first, in a form that shows the campaign, its lists and segments with their "+
			"contact counts, its exclusions, and the send time; nothing is scheduled without their yes. When this "+
			"client cannot show the form, nothing is scheduled and the result holds dashboard_url, where the user "+
			"schedules the campaign. A campaign with issues, or with a segment that is not ready, is refused; "+
			"get_campaign and get_segment show them. To change the time, use "+
			"unschedule_campaign and schedule again. The result says whether the schedule was accepted.",
			true),
		newTool(ToolInfo{
			Name:           "unschedule_campaign",
			Title:          "Unschedule campaign",
			Toolset:        ToolsetCampaigns,
			RequiredScopes: []string{"campaigns:send"},
			Annotations:    annWrite(),
		}, "Returns a validating or scheduled campaign to draft, cancelling its scheduled send. A campaign that "+
			"has queued mail, or is in any other status, cannot be unscheduled. Use it to stop a send or change "+
			"its time. The result is the campaign, without its content.",
			unscheduleCampaign),
		newTool(ToolInfo{
			Name:           "get_campaign_stats",
			Title:          "Get campaign statistics",
			Toolset:        ToolsetCampaigns,
			RequiredScopes: []string{"campaigns:read"},
			Annotations:    annRead(),
		}, "Returns a campaign's live counters (recipients, deliveries, opens, clicks, unsubscribes, "+
			"complaints), its stored report of recipients by status and bounce classification, and its rates "+
			"in percent. Use it to report how a sent campaign performed. The report is recounted from time to time: a call that finds it missing or old asks "+
			"for a recount and returns what is stored, so call again later for new numbers. "+
			"list_campaign_recipients lists the recipients behind a report count.",
			getCampaignStats),
		newTool(ToolInfo{
			Name:           "list_campaign_recipients",
			Title:          "List campaign recipients",
			Toolset:        ToolsetCampaigns,
			RequiredScopes: []string{"campaigns:read"},
			Annotations:    annRead(),
		}, "Lists a campaign's recipients in one status or with one bounce classification, as keys of "+
			"get_campaign_stats' report name them, with their bounce details. Give exactly one of status and "+
			"bounce_classification. Use it to see which recipients bounced or failed. The result holds one page and a "+
			"next_cursor for the page after it.",
			listCampaignRecipients),
	}
}

// campaignIDInput names one campaign.
type campaignIDInput struct {
	CampaignID string `json:"campaign_id" jsonschema:"Campaign ID."`
}

func (in campaignIDInput) id() (uuid.UUID, error) {
	return parseID("campaign_id", in.CampaignID)
}

// campaignSummary is a campaign as list_campaigns shows it.
type campaignSummary struct {
	ID            string        `json:"id" jsonschema:"Campaign ID."`
	Name          string        `json:"name" jsonschema:"Campaign name; recipients do not see it."`
	Status        string        `json:"status" jsonschema:"draft, validating, waiting_for_review, scheduled, sending, paused, sent, canceled or failed. Only a draft can be changed, sent or deleted."`
	StatusReason  string        `json:"status_reason,omitempty" jsonschema:"Why the campaign is in its status."`
	Subject       string        `json:"subject,omitempty" jsonschema:"Subject line."`
	Preheader     string        `json:"preheader,omitempty" jsonschema:"Preview text."`
	From          *senderOutput `json:"from,omitempty" jsonschema:"Sender; missing until a complete sender is set, or when its domain was deleted."`
	ReplyTo       string        `json:"reply_to,omitempty" jsonschema:"Reply-to address."`
	SendAt        *time.Time    `json:"send_at,omitempty" jsonschema:"When the campaign sends or sent."`
	SendResumesAt *time.Time    `json:"send_resumes_at,omitempty" jsonschema:"When a send that reached the account's daily marketing limit continues."`
	FinishedAt    *time.Time    `json:"finished_at,omitempty" jsonschema:"When the send ended."`
	CreatedAt     time.Time     `json:"created_at" jsonschema:"When the campaign was created."`
	UpdatedAt     time.Time     `json:"updated_at" jsonschema:"When the campaign's own fields last changed."`
}

func campaignSummaryFrom(c *responses.CampaignListItem) campaignSummary {
	return campaignSummary{
		ID:            c.ID.String(),
		Name:          c.Name,
		Status:        c.Status,
		StatusReason:  deref(c.StatusReason),
		Subject:       deref(c.Subject),
		Preheader:     deref(c.Preheader),
		From:          senderFrom(c.From),
		ReplyTo:       deref(c.ReplyTo),
		SendAt:        c.SendAt,
		SendResumesAt: c.SendResumesAt,
		FinishedAt:    c.FinishedAt,
		CreatedAt:     c.CreatedAt,
		UpdatedAt:     c.UpdatedAt,
	}
}

// campaignIssue is a reason a draft cannot be sent yet.
type campaignIssue struct {
	Code    string `json:"code" jsonschema:"sender_missing, lists_missing, subject_missing, preheader_missing, design_missing, text_broken or unsubscribe_link_missing."`
	Message string `json:"message" jsonschema:"What to do about it."`
}

// campaignResult is a campaign as a write returns it: with its lists and
// issues, but without its content.
type campaignResult struct {
	campaignSummary
	ListIDs           []string        `json:"list_ids" jsonschema:"The lists the campaign sends to, in order."`
	SegmentIDs        []string        `json:"segment_ids" jsonschema:"The segments the campaign sends to."`
	ExcludeListIDs    []string        `json:"exclude_list_ids" jsonschema:"The lists whose contacts get nothing from the campaign."`
	ExcludeSegmentIDs []string        `json:"exclude_segment_ids" jsonschema:"The segments whose members get nothing from the campaign."`
	Editor            string          `json:"editor,omitempty" jsonschema:"The design's editor: advanced (MJML), html, or simple (the Simple Editor, written here as AhaML in markup). Missing while the campaign has no design."`
	Ready             *bool           `json:"ready,omitempty" jsonschema:"For a draft: true when it has no issue and can be sent."`
	Issues            []campaignIssue `json:"issues,omitempty" jsonschema:"For a draft: what keeps it from being sent."`
	MarkupWarnings    []markupWarning `json:"markup_warnings,omitempty" jsonschema:"Warnings about the markup this write stored; only a write with markup has them."`
}

func campaignResultFrom(c *responses.Campaign) campaignResult {
	out := campaignResult{
		campaignSummary:   campaignSummaryFrom(&c.CampaignListItem),
		ListIDs:           idStrings(c.ListIDs),
		SegmentIDs:        idStrings(c.SegmentIDs),
		ExcludeListIDs:    idStrings(c.ExcludeListIDs),
		ExcludeSegmentIDs: idStrings(c.ExcludeSegmentIDs),
		Editor:            deref(c.Editor),
		Ready:             c.Ready,
		MarkupWarnings:    markupWarningsFrom(c.MarkupWarnings),
	}
	for _, issue := range c.Issues {
		out.Issues = append(out.Issues, campaignIssue{Code: issue.Code, Message: issue.Message})
	}
	return out
}

// campaignDetail is a campaign with its content.
type campaignDetail struct {
	campaignResult
	Content *templateContent `json:"content,omitempty" jsonschema:"The design's content; missing while the campaign has no design."`
}

type listCampaignsInput struct {
	pageInput
	Status string `json:"status,omitempty" jsonschema:"Only campaigns in this status: draft, validating, waiting_for_review, scheduled, sending, paused, sent, canceled or failed."`
}

type listCampaignsOutput struct {
	Campaigns []campaignSummary `json:"campaigns" jsonschema:"This page's campaigns."`
	pageOutput
}

func listCampaigns(ctx context.Context, env *toolEnv, in listCampaignsInput) (listCampaignsOutput, error) {
	page, err := in.params()
	if err != nil {
		return listCampaignsOutput{}, err
	}
	resp, _, err := env.client.CampaignsAPI.ListCampaigns(ctx, env.accountID, requests.ListCampaignsParams{
		Status: optionalString(in.Status),
		Limit:  page.Limit,
		After:  page.After,
	})
	if err != nil {
		return listCampaignsOutput{}, err
	}
	out := listCampaignsOutput{Campaigns: []campaignSummary{}, pageOutput: pageOutputFrom(resp.Pagination)}
	for i := range resp.Data {
		out.Campaigns = append(out.Campaigns, campaignSummaryFrom(&resp.Data[i]))
	}
	return out, nil
}

func getCampaign(ctx context.Context, env *toolEnv, in campaignIDInput) (campaignDetail, error) {
	id, err := in.id()
	if err != nil {
		return campaignDetail{}, err
	}
	c, _, err := env.client.CampaignsAPI.GetCampaign(ctx, env.accountID, id)
	if err != nil {
		return campaignDetail{}, err
	}
	return campaignDetail{campaignResult: campaignResultFrom(c), Content: templateContentFrom(c.Content)}, nil
}

// idStrings returns IDs as strings, never nil.
func idStrings(ids []uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}

// parseIDs reads an ID list argument named field. It keeps nil, an omitted
// argument, apart from an empty list.
func parseIDs(field string, values []string) ([]uuid.UUID, error) {
	if values == nil {
		return nil, nil
	}
	ids := make([]uuid.UUID, 0, len(values))
	for _, v := range values {
		id, err := parseID(field, v)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// audienceInput is the audience a campaign write takes. On an update, each
// field omitted keeps its set and an empty one empties it.
type audienceInput struct {
	ListIDs           []string `json:"list_ids,omitempty" jsonschema:"IDs of the lists to send to, in order, up to 50 lists in all with exclude_list_ids. On update_campaign it replaces the lists; [] removes them."`
	SegmentIDs        []string `json:"segment_ids,omitempty" jsonschema:"IDs of the segments to send to, up to 10 segments in all with exclude_segment_ids. On update_campaign it replaces the segments; [] removes them."`
	ExcludeListIDs    []string `json:"exclude_list_ids,omitempty" jsonschema:"IDs of lists whose contacts get nothing, whatever their membership status, also when an included list or segment reaches them. On update_campaign it replaces the excluded lists; [] removes them."`
	ExcludeSegmentIDs []string `json:"exclude_segment_ids,omitempty" jsonschema:"IDs of segments whose members get nothing, also when an included list or segment reaches them. On update_campaign it replaces the excluded segments; [] removes them."`
}

// audienceIDs is a read audienceInput.
type audienceIDs struct {
	lists, segments, excludeLists, excludeSegments []uuid.UUID
}

func (in audienceInput) ids() (audienceIDs, error) {
	var out audienceIDs
	var err error
	if out.lists, err = parseIDs("list_ids", in.ListIDs); err != nil {
		return audienceIDs{}, err
	}
	if out.segments, err = parseIDs("segment_ids", in.SegmentIDs); err != nil {
		return audienceIDs{}, err
	}
	if out.excludeLists, err = parseIDs("exclude_list_ids", in.ExcludeListIDs); err != nil {
		return audienceIDs{}, err
	}
	if out.excludeSegments, err = parseIDs("exclude_segment_ids", in.ExcludeSegmentIDs); err != nil {
		return audienceIDs{}, err
	}
	return out, nil
}

type createCampaignInput struct {
	Name      string        `json:"name" jsonschema:"Campaign name; recipients do not see it."`
	Subject   string        `json:"subject,omitempty" jsonschema:"Subject line."`
	Preheader string        `json:"preheader,omitempty" jsonschema:"Preview text."`
	From      *addressInput `json:"from,omitempty" jsonschema:"Sender, on a domain of this account with valid DNS records."`
	ReplyTo   string        `json:"reply_to,omitempty" jsonschema:"Reply-to address, without a display name."`
	audienceInput
	Editor string `json:"editor,omitempty" jsonschema:"advanced, html or simple. Alone, it makes an empty design; with content it must match it: simple for markup, advanced for mjml, html for html alone. Not with template_id."`
	contentInput
	TemplateID string `json:"template_id,omitempty" jsonschema:"ID of a campaign template whose design the campaign copies. Not with editor or content."`
}

func createCampaign(ctx context.Context, env *toolEnv, in createCampaignInput) (campaignResult, error) {
	audience, err := in.ids()
	if err != nil {
		return campaignResult{}, err
	}
	req := requests.CreateCampaignRequest{
		Name:              in.Name,
		Subject:           optionalString(in.Subject),
		Preheader:         optionalString(in.Preheader),
		From:              senderPtr(in.From),
		ReplyTo:           optionalString(strings.TrimSpace(in.ReplyTo)),
		ListIDs:           audience.lists,
		SegmentIDs:        audience.segments,
		ExcludeListIDs:    audience.excludeLists,
		ExcludeSegmentIDs: audience.excludeSegments,
		Editor:            optionalString(in.Editor),
		Content:           in.request(),
	}
	if in.TemplateID != "" {
		templateID, err := parseID("template_id", in.TemplateID)
		if err != nil {
			return campaignResult{}, err
		}
		req.TemplateID = &templateID
	}
	c, _, err := env.client.CampaignsAPI.CreateCampaign(ctx, env.accountID, req)
	if err != nil {
		return campaignResult{}, err
	}
	return campaignResultFrom(c), nil
}

type updateCampaignInput struct {
	CampaignID string        `json:"campaign_id" jsonschema:"Campaign ID."`
	Name       string        `json:"name,omitempty" jsonschema:"New name."`
	Subject    *string       `json:"subject,omitempty" jsonschema:"New subject line. An empty value clears it."`
	Preheader  *string       `json:"preheader,omitempty" jsonschema:"New preview text. An empty value clears it."`
	From       *addressInput `json:"from,omitempty" jsonschema:"New sender, address and name together. An empty email clears it; the reply-to is kept."`
	ReplyTo    *string       `json:"reply_to,omitempty" jsonschema:"New reply-to address, without a display name. An empty value clears it."`
	audienceInput
	contentUpdateInput
}

func updateCampaign(ctx context.Context, env *toolEnv, in updateCampaignInput) (campaignResult, error) {
	id, err := parseID("campaign_id", in.CampaignID)
	if err != nil {
		return campaignResult{}, err
	}
	audience, err := in.ids()
	if err != nil {
		return campaignResult{}, err
	}
	content, err := in.request()
	if err != nil {
		return campaignResult{}, err
	}
	req := requests.UpdateCampaignRequest{
		Name: optionalString(in.Name),
		// ahasend-go sends an empty subject, preview text or reply-to as
		// null, which clears it.
		Subject:           in.Subject,
		Preheader:         in.Preheader,
		From:              addressUpdate(in.From),
		ReplyTo:           trimmed(in.ReplyTo),
		ListIDs:           audience.lists,
		SegmentIDs:        audience.segments,
		ExcludeListIDs:    audience.excludeLists,
		ExcludeSegmentIDs: audience.excludeSegments,
		Content:           content,
	}
	c, _, err := env.client.CampaignsAPI.UpdateCampaign(ctx, env.accountID, id, req)
	if err != nil {
		return campaignResult{}, err
	}
	return campaignResultFrom(c), nil
}

func deleteCampaign(ctx context.Context, env *toolEnv, in campaignIDInput) (confirmationOutput, error) {
	id, err := in.id()
	if err != nil {
		return confirmationOutput{}, err
	}
	resp, _, err := env.client.CampaignsAPI.DeleteCampaign(ctx, env.accountID, id)
	if err != nil {
		return confirmationOutput{}, err
	}
	return confirmationOutput{Message: resp.Message}, nil
}

type duplicateCampaignInput struct {
	campaignIDInput
	Name string `json:"name,omitempty" jsonschema:"The copy's name. Default: Copy of and the campaign's name."`
}

func duplicateCampaign(ctx context.Context, env *toolEnv, in duplicateCampaignInput) (campaignResult, error) {
	id, err := in.id()
	if err != nil {
		return campaignResult{}, err
	}
	c, _, err := env.client.CampaignsAPI.DuplicateCampaign(ctx, env.accountID, id,
		requests.DuplicateCampaignRequest{Name: optionalString(in.Name)})
	if err != nil {
		return campaignResult{}, err
	}
	return campaignResultFrom(c), nil
}

type sendCampaignTestInput struct {
	campaignIDInput
	To []string `json:"to" jsonschema:"1 to 5 email addresses to send the test to."`
}

type testFailure struct {
	Email   string `json:"email" jsonschema:"Address the mail server refused."`
	Message string `json:"message" jsonschema:"Why it was refused."`
}

type sendCampaignTestOutput struct {
	SentTo        []string      `json:"sent_to" jsonschema:"Addresses the mail server accepted."`
	Failed        []testFailure `json:"failed" jsonschema:"Addresses the mail server refused."`
	ResultUnknown bool          `json:"result_unknown" jsonschema:"True when the answer could not be read: the test may have gone out."`
}

func sendCampaignTest(ctx context.Context, env *toolEnv, in sendCampaignTestInput) (sendCampaignTestOutput, error) {
	id, err := in.id()
	if err != nil {
		return sendCampaignTestOutput{}, err
	}
	resp, _, err := env.client.CampaignsAPI.TestCampaign(ctx, env.accountID, id, requests.TestCampaignRequest{To: in.To})
	if err != nil {
		return sendCampaignTestOutput{}, campaignNotReadyError(err)
	}
	out := sendCampaignTestOutput{SentTo: nonNil(resp.SentTo), Failed: []testFailure{}, ResultUnknown: resp.ResultUnknown}
	for _, f := range resp.Failed {
		out.Failed = append(out.Failed, testFailure{Email: f.Email, Message: f.Message})
	}
	return out, nil
}

// campaignNotReadyError adds the issues of a campaign the API refused as not
// ready to the error text, so the agent knows what to fix.
func campaignNotReadyError(err error) error {
	notReady, ok := api.AsCampaignNotReady(err)
	if !ok {
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "AhaSend refused the request (HTTP 400): %s. Fix these first:", strings.TrimRight(notReady.Message, ". "))
	for _, issue := range notReady.Issues {
		fmt.Fprintf(&b, " %s (%s).", strings.TrimRight(issue.Message, ". "), issue.Code)
	}
	return &toolError{kind: ErrorKindAPI4xx, message: b.String()}
}

func unscheduleCampaign(ctx context.Context, env *toolEnv, in campaignIDInput) (campaignResult, error) {
	id, err := in.id()
	if err != nil {
		return campaignResult{}, err
	}
	c, _, err := env.client.CampaignsAPI.UnscheduleCampaign(ctx, env.accountID, id)
	if err != nil {
		return campaignResult{}, err
	}
	return campaignResultFrom(c), nil
}

// campaignReport is a campaign's stored report.
type campaignReport struct {
	ComputedAt             time.Time      `json:"computed_at" jsonschema:"When the report was counted."`
	Recipients             int            `json:"recipients" jsonschema:"Recipients the report counted."`
	Openers                int            `json:"openers" jsonschema:"Contacts who opened, each once, without bots."`
	Clickers               int            `json:"clickers" jsonschema:"Contacts who clicked, each once, without bots."`
	Bounces                int            `json:"bounces" jsonschema:"Recipients that bounced or failed."`
	Deferrals              int            `json:"deferrals" jsonschema:"Recipients whose delivery was deferred."`
	ByStatus               map[string]int `json:"by_status" jsonschema:"Recipients by status; each key is a status of list_campaign_recipients."`
	ByBounceClassification map[string]int `json:"by_bounce_classification" jsonschema:"Bounces by classification; each key but Unknown is a bounce_classification of list_campaign_recipients."`
}

// campaignRates are a campaign's rates in percent.
type campaignRates struct {
	OpenRate     *float64 `json:"open_rate,omitempty" jsonschema:"Openers out of delivered messages."`
	ClickRate    *float64 `json:"click_rate,omitempty" jsonschema:"Clickers out of delivered messages."`
	BounceRate   *float64 `json:"bounce_rate,omitempty" jsonschema:"Bounces out of the report's recipients."`
	DeferralRate *float64 `json:"deferral_rate,omitempty" jsonschema:"Deferrals out of the report's recipients."`
}

type campaignStatsOutput struct {
	CampaignID    string          `json:"campaign_id" jsonschema:"Campaign ID."`
	Recipients    int             `json:"recipients" jsonschema:"Recipients the campaign sends to."`
	Delivered     int             `json:"delivered" jsonschema:"Messages delivered."`
	Opens         int             `json:"opens" jsonschema:"Opens, without bots."`
	Clicks        int             `json:"clicks" jsonschema:"Clicks, without bots."`
	Unsubscribes  int             `json:"unsubscribes" jsonschema:"Recipients who unsubscribed."`
	Complaints    int             `json:"complaints" jsonschema:"Recipients who marked the email as spam."`
	Report        *campaignReport `json:"report,omitempty" jsonschema:"The stored report; missing until the first count."`
	Rates         campaignRates   `json:"rates" jsonschema:"Rates in percent; a rate is missing while its base is zero."`
	RecountFailed bool            `json:"recount_failed" jsonschema:"True while the last recount failed, so the report may be old."`
}

func getCampaignStats(ctx context.Context, env *toolEnv, in campaignIDInput) (campaignStatsOutput, error) {
	id, err := in.id()
	if err != nil {
		return campaignStatsOutput{}, err
	}
	s, _, err := env.client.CampaignsAPI.GetCampaignStats(ctx, env.accountID, id)
	if err != nil {
		return campaignStatsOutput{}, err
	}
	out := campaignStatsOutput{
		CampaignID:   s.CampaignID.String(),
		Recipients:   s.Recipients,
		Delivered:    s.Delivered,
		Opens:        s.Opens,
		Clicks:       s.Clicks,
		Unsubscribes: s.Unsubscribes,
		Complaints:   s.Complaints,
		Rates: campaignRates{
			OpenRate:     s.Rates.OpenRate,
			ClickRate:    s.Rates.ClickRate,
			BounceRate:   s.Rates.BounceRate,
			DeferralRate: s.Rates.DeferralRate,
		},
		RecountFailed: s.RecountFailed,
	}
	if r := s.Report; r != nil {
		out.Report = &campaignReport{
			ComputedAt:             r.ComputedAt,
			Recipients:             r.Recipients,
			Openers:                r.Openers,
			Clickers:               r.Clickers,
			Bounces:                r.Bounces,
			Deferrals:              r.Deferrals,
			ByStatus:               r.ByStatus,
			ByBounceClassification: r.ByBounceClassification,
		}
	}
	return out, nil
}

type listCampaignRecipientsInput struct {
	pageInput
	campaignIDInput
	Status               string `json:"status,omitempty" jsonschema:"A key of the report's by_status, such as Delivered or Bounced. Give status or bounce_classification."`
	BounceClassification string `json:"bounce_classification,omitempty" jsonschema:"A key of the report's by_bounce_classification other than Unknown. Give status or bounce_classification."`
}

type campaignRecipient struct {
	ContactID            string    `json:"contact_id" jsonschema:"Contact ID."`
	Email                string    `json:"email" jsonschema:"Address the campaign sent to."`
	ListName             string    `json:"list_name,omitempty" jsonschema:"The list through which the contact got the campaign; missing when it came through a segment."`
	SegmentID            string    `json:"segment_id,omitempty" jsonschema:"The segment through which the contact got the campaign; missing when it came through a list, which a contact on a list and a segment did."`
	SegmentName          string    `json:"segment_name,omitempty" jsonschema:"The segment's name at the send."`
	Status               string    `json:"status" jsonschema:"Status, as the report names it."`
	BounceClassification string    `json:"bounce_classification,omitempty" jsonschema:"Bounce classification."`
	BounceReason         string    `json:"bounce_reason,omitempty" jsonschema:"Bounce reason."`
	BounceMessage        string    `json:"bounce_message,omitempty" jsonschema:"The receiving server's bounce message."`
	Attempts             int       `json:"attempts" jsonschema:"Delivery attempts."`
	UpdatedAt            time.Time `json:"updated_at" jsonschema:"When the recipient last changed."`
}

type listCampaignRecipientsOutput struct {
	Recipients []campaignRecipient `json:"recipients" jsonschema:"This page's recipients."`
	pageOutput
}

func listCampaignRecipients(ctx context.Context, env *toolEnv, in listCampaignRecipientsInput) (listCampaignRecipientsOutput, error) {
	id, err := in.id()
	if err != nil {
		return listCampaignRecipientsOutput{}, err
	}
	if (in.Status == "") == (in.BounceClassification == "") {
		return listCampaignRecipientsOutput{}, invalidInput("Give status or bounce_classification, not both and not neither.")
	}
	page, err := in.params()
	if err != nil {
		return listCampaignRecipientsOutput{}, err
	}
	resp, _, err := env.client.CampaignsAPI.ListCampaignRecipients(ctx, env.accountID, id, requests.ListCampaignRecipientsParams{
		Status:               optionalString(in.Status),
		BounceClassification: optionalString(in.BounceClassification),
		Limit:                page.Limit,
		After:                page.After,
	})
	if err != nil {
		return listCampaignRecipientsOutput{}, err
	}
	out := listCampaignRecipientsOutput{Recipients: []campaignRecipient{}, pageOutput: pageOutputFrom(resp.Pagination)}
	for _, r := range resp.Data {
		recipient := campaignRecipient{
			ContactID:            r.ContactID.String(),
			Email:                r.Email,
			ListName:             deref(r.ListName),
			SegmentName:          deref(r.SegmentName),
			Status:               r.Status,
			BounceClassification: deref(r.BounceClassification),
			BounceReason:         deref(r.BounceReason),
			BounceMessage:        deref(r.BounceMessage),
			Attempts:             r.Attempts,
			UpdatedAt:            r.UpdatedAt,
		}
		if r.SegmentID != nil {
			recipient.SegmentID = r.SegmentID.String()
		}
		out.Recipients = append(out.Recipients, recipient)
	}
	return out, nil
}

// campaignSenderText formats a campaign's sender for the confirmation form.
func campaignSenderText(from *common.SenderAddress) string {
	if from == nil || from.Email == "" {
		return "(none)"
	}
	if name := deref(from.Name); name != "" {
		return fmt.Sprintf("%s <%s>", name, from.Email)
	}
	return from.Email
}

// trimmed returns the value without surrounding spaces, keeping nil.
func trimmed(s *string) *string {
	if s == nil {
		return nil
	}
	return ptr(strings.TrimSpace(*s))
}

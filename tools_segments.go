package ahasendmcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
)

// segmentMembershipNote says how a segment's members and counts behave. Each
// segment tool that returns a segment holds it.
const segmentMembershipNote = "AhaSend keeps a segment's members up to date by itself as contacts, list " +
	"memberships and campaign activity change, usually within a minute; rules about campaigns follow within an " +
	"hour of a send. member_count counts every contact that matches the rules, whatever its consent; a campaign " +
	"reaches only the enabled members who have not unsubscribed, were not found invalid, and are confirmed on " +
	"at least one list or on no list at all, which get_segment counts as marketable_count. A segment whose " +
	"state is building is still finding its members: it has no marketable_count yet, its member_count and " +
	"members may still follow its previous rules, and a campaign that uses it cannot be sent until it is ready."

// segmentRulesNote says how to write a segment definition. Each tool that
// takes a definition holds it.
const segmentRulesNote = "A definition has match (all or any) and conditions: 1 to 50 conditions in all, at " +
	"most 5 of them on campaign, any_campaign or campaign_count. Each entry of conditions is a condition " +
	"{field, op, value} or a group {match, conditions} whose conditions cannot be groups. The fields and their " +
	"operators: email, first_name, last_name and text attributes take equal, not_equal, contains, not_contains, " +
	"begins_with, not_begins_with, ends_with, not_ends_with, is_empty and is_not_empty. Number attributes take " +
	"equal, not_equal, less, less_or_equal, greater and greater_or_equal. created_at (when the contact was " +
	"created) and date and timestamp attributes take equal, not_equal, before, before_or_on, after and " +
	"after_or_on, with a date such as 2024-06-15 or a relative time such as today or 30 days ago; created_at " +
	"and timestamps also take a time such as 2 hours from now. Boolean attributes take is_true and is_false. " +
	"Date, number, timestamp and boolean attributes also take is_null and is_not_null. Date attributes also " +
	"take month_is with a month from 1 to 12, and anniversary_within with a number of days from 0 to 365 (the " +
	"month and day fall on today or one of the next N days). list takes in (a confirmed member of the list) or " +
	"not_in, with a list ID. campaign takes received, not_received, opened, not_opened, clicked or " +
	"not_clicked, with a campaign ID. any_campaign takes received, opened, clicked, not_opened or not_clicked, " +
	`with a relative time such as 14 days ago. campaign_count takes received_at_least, received_fewer_than, ` +
	`opened_at_least, opened_fewer_than, clicked_at_least or clicked_fewer_than, with {"count": 3, "since": ` +
	`"90 days ago"}. A contact field is attributes.<key>, for a contact field defined in the dashboard. is_empty, ` +
	"is_not_empty, is_null, is_not_null, is_true and is_false take no value. A negated operator also matches a " +
	"contact without a value. Opens and clicks by bots do not count. Relative times and days are read in " +
	"timezone. A refused definition is answered with each problem named by its path, such as " +
	"conditions[1].conditions[0].value."

// segmentUnsubscribeNote says what an unsubscribe from a segment send does.
const segmentUnsubscribeNote = "A contact who unsubscribes from a campaign sent to a segment, or reports it as " +
	"spam, gets no more marketing email from the account."

func segmentsTools() []tool {
	return []tool{
		newTool(ToolInfo{
			Name:           "list_segments",
			Title:          "List segments",
			Toolset:        ToolsetAudience,
			RequiredScopes: []string{"segments:read"},
			Annotations:    annRead(),
		}, "Lists the segments of this account, newest first, with their state and member_count but not their "+
			"rules. A segment is a saved set of rules over the account's contacts. Use it to find a segment; "+
			"get_segment returns one with its rules and marketable_count. "+segmentMembershipNote+" The result "+
			"holds one page and a next_cursor for the page after it.",
			listSegments),
		newTool(ToolInfo{
			Name:           "get_segment",
			Title:          "Get segment",
			Toolset:        ToolsetAudience,
			RequiredScopes: []string{"segments:read"},
			Annotations:    annRead(),
		}, "Returns one segment with its rules, state, member_count and marketable_count: the members a "+
			"campaign to this segment alone would reach. Use it to check a segment before a campaign is sent to "+
			"it, and to see whether a segment being built is ready. "+segmentMembershipNote+" marketable_count "+
			"is also missing when the members cannot be counted in time; call again later.",
			getSegment),
		newTool(ToolInfo{
			Name:           "create_segment",
			Title:          "Create segment",
			Toolset:        ToolsetAudience,
			RequiredScopes: []string{"segments:write"},
			Annotations:    annWrite(),
		}, "Saves a segment: rules over the account's contacts that AhaSend keeps its members matched to. Use "+
			"preview_segment first to check that the rules match the contacts the user means. The new segment "+
			"is building until its members are found, which can take some minutes on a large account; "+
			"get_segment shows when it is ready. "+segmentRulesNote+" "+segmentUnsubscribeNote+" The result is "+
			"the new segment.",
			createSegment),
		newTool(ToolInfo{
			Name:           "update_segment",
			Title:          "Update segment",
			Toolset:        ToolsetAudience,
			RequiredScopes: []string{"segments:write"},
			Annotations:    annWrite(),
		}, "Changes a segment's name, description, rules or time zone. Only the fields given change; a "+
			"definition replaces the rules as a whole. New rules or a new time zone build the segment's members "+
			"again, and it is building until that ends; a new name or description leaves its members as they "+
			"are. A failed segment is built again by any update, one with only segment_id included. While the "+
			"segment is being changed elsewhere the update is refused with 409 and nothing changes: call again "+
			"in a minute. "+segmentRulesNote+" The result is the changed segment.",
			updateSegment),
		newTool(ToolInfo{
			Name:           "delete_segment",
			Title:          "Delete segment",
			Toolset:        ToolsetAudience,
			RequiredScopes: []string{"segments:delete"},
			Annotations:    annDestructive(),
		}, "Deletes a segment; its contacts stay in the account. A segment that a campaign still needs, "+
			"included or excluded, cannot be deleted. Use it only when the user asks to remove the segment. The "+
			"result is AhaSend's confirmation.",
			deleteSegment),
		newTool(ToolInfo{
			Name:           "preview_segment",
			Title:          "Preview segment",
			Toolset:        ToolsetAudience,
			RequiredScopes: []string{"segments:read", "contacts:read"},
			Annotations:    annRead(),
		}, "Tries a segment definition without saving anything: how many contacts it matches now, whatever "+
			"their consent, and up to 10 of them. Use it before create_segment or update_segment to check the "+
			"rules. count is missing when the contacts cannot be counted in time. "+segmentRulesNote,
			previewSegment),
		newTool(ToolInfo{
			Name:           "list_segment_contacts",
			Title:          "List a segment's contacts",
			Toolset:        ToolsetAudience,
			RequiredScopes: []string{"segments:read", "contacts:read"},
			Annotations:    annRead(),
		}, "Lists the members of one segment: each contact's ID, email address and when it became a member. "+
			"Members match the rules whatever their consent, so not every member gets campaigns. While the "+
			"segment is building, its members may still follow its previous rules. get_contact returns a "+
			"contact's other fields. The result holds one page and a next_cursor for the page after it.",
			listSegmentContacts),
		newTool(ToolInfo{
			Name:           "list_contact_segments",
			Title:          "List a contact's segments",
			Toolset:        ToolsetAudience,
			RequiredScopes: []string{"segments:read", "contacts:read"},
			Annotations:    annRead(),
		}, "Lists the segments one contact, found by ID or email address, is a member of. A segment that is "+
			"building may still list the contact by its previous rules. The result holds one page and a "+
			"next_cursor for the page after it.",
			listContactSegments),
	}
}

// segmentDefinition is a segment's rules, as the segment tools take and
// return them.
type segmentDefinition struct {
	Match      string        `json:"match" jsonschema:"all: a contact must match every entry of conditions; any: at least one."`
	Conditions []segmentRule `json:"conditions" jsonschema:"The conditions and groups of conditions, at least one."`
}

// segmentRule is one entry of a definition's conditions: a condition or a
// group.
type segmentRule struct {
	Field      string             `json:"field,omitempty" jsonschema:"For a condition: email, first_name, last_name, created_at, attributes.<key>, list, campaign, any_campaign or campaign_count."`
	Op         string             `json:"op,omitempty" jsonschema:"For a condition: an operator that applies to field."`
	Value      any                `json:"value,omitempty" jsonschema:"For a condition: a string, number or boolean, or a {\"count\", \"since\"} object for campaign_count. Omit it for an operator that takes no value."`
	Match      string             `json:"match,omitempty" jsonschema:"For a group: all or any."`
	Conditions []segmentCondition `json:"conditions,omitempty" jsonschema:"For a group: its conditions, at least one; a group cannot hold a group."`
}

// segmentCondition is one condition of a group.
type segmentCondition struct {
	Field string `json:"field" jsonschema:"email, first_name, last_name, created_at, attributes.<key>, list, campaign, any_campaign or campaign_count."`
	Op    string `json:"op" jsonschema:"An operator that applies to field."`
	Value any    `json:"value,omitempty" jsonschema:"A string, number or boolean, or a {\"count\", \"since\"} object for campaign_count. Omit it for an operator that takes no value."`
}

// segmentCount is the value of a campaign_count condition.
type segmentCount struct {
	Count int    `json:"count"`
	Since string `json:"since"`
}

// request turns the definition into the API's form. It refuses an entry that
// is both a condition and a group, or neither, and a value that is not a
// string, number, boolean or count object.
func (d *segmentDefinition) request() (common.SegmentDefinition, error) {
	out := common.SegmentDefinition{
		SchemaVersion: common.SegmentSchemaVersion,
		Match:         d.Match,
		Conditions:    make([]common.SegmentRule, 0, len(d.Conditions)),
	}
	for i, r := range d.Conditions {
		path := fmt.Sprintf("conditions[%d]", i)
		group := r.Match != "" || r.Conditions != nil
		condition := r.Field != "" || r.Op != "" || r.Value != nil
		if group == condition {
			return common.SegmentDefinition{}, invalidInput("%s must be a condition, with field and op, or a group, "+
				"with match and conditions.", path)
		}
		if condition {
			c, err := segmentConditionRequest(path, segmentCondition{Field: r.Field, Op: r.Op, Value: r.Value})
			if err != nil {
				return common.SegmentDefinition{}, err
			}
			out.Conditions = append(out.Conditions, common.SegmentRule{Condition: &c})
			continue
		}
		g := common.SegmentGroup{Match: r.Match, Conditions: make([]common.SegmentCondition, 0, len(r.Conditions))}
		for j, gc := range r.Conditions {
			c, err := segmentConditionRequest(fmt.Sprintf("%s.conditions[%d]", path, j), gc)
			if err != nil {
				return common.SegmentDefinition{}, err
			}
			g.Conditions = append(g.Conditions, c)
		}
		out.Conditions = append(out.Conditions, common.SegmentRule{Group: &g})
	}
	return out, nil
}

func segmentConditionRequest(path string, c segmentCondition) (common.SegmentCondition, error) {
	out := common.SegmentCondition{Field: strings.TrimSpace(c.Field), Op: strings.TrimSpace(c.Op)}
	if c.Value == nil {
		return out, nil
	}
	raw, err := json.Marshal(c.Value)
	if err != nil {
		return common.SegmentCondition{}, invalidInput("%s.value must be a string, number or boolean, or a count "+
			"object.", path)
	}
	var value common.SegmentValue
	if err := json.Unmarshal(raw, &value); err != nil {
		return common.SegmentCondition{}, invalidInput("%s.value must be a string, number or boolean, or a "+
			`{"count", "since"} object for campaign_count.`, path)
	}
	out.Value = &value
	return out, nil
}

// segmentDefinitionFrom shows the API's definition as the tools take it.
func segmentDefinitionFrom(d common.SegmentDefinition) segmentDefinition {
	out := segmentDefinition{Match: d.Match, Conditions: make([]segmentRule, 0, len(d.Conditions))}
	for _, r := range d.Conditions {
		if r.Group != nil {
			rule := segmentRule{Match: r.Group.Match, Conditions: make([]segmentCondition, 0, len(r.Group.Conditions))}
			for _, c := range r.Group.Conditions {
				rule.Conditions = append(rule.Conditions, segmentConditionFrom(c))
			}
			out.Conditions = append(out.Conditions, rule)
			continue
		}
		if r.Condition != nil {
			c := segmentConditionFrom(*r.Condition)
			out.Conditions = append(out.Conditions, segmentRule{Field: c.Field, Op: c.Op, Value: c.Value})
		}
	}
	return out
}

func segmentConditionFrom(c common.SegmentCondition) segmentCondition {
	return segmentCondition{Field: c.Field, Op: c.Op, Value: segmentValueFrom(c.Value)}
}

func segmentValueFrom(v *common.SegmentValue) any {
	switch {
	case v == nil:
		return nil
	case v.String != nil:
		return *v.String
	case v.Number != nil:
		return *v.Number
	case v.Bool != nil:
		return *v.Bool
	case v.Count != nil:
		return segmentCount{Count: v.Count.Count, Since: v.Count.Since}
	}
	return nil
}

// segmentSummary is a segment without its rules.
type segmentSummary struct {
	ID              string     `json:"id" jsonschema:"Segment ID."`
	Name            string     `json:"name" jsonschema:"Segment name; names are not unique."`
	Description     string     `json:"description,omitempty" jsonschema:"Description."`
	State           string     `json:"state" jsonschema:"building while the members are found, ready, or failed. A campaign that uses the segment can be sent only when it is ready."`
	FailureReason   string     `json:"failure_reason,omitempty" jsonschema:"Why the segment failed; any update_segment builds it again."`
	MemberCount     int        `json:"member_count" jsonschema:"Contacts that match the rules, whatever their consent; not all of them get campaigns."`
	Timezone        string     `json:"timezone" jsonschema:"The IANA time zone that relative times and days in the rules are read in."`
	LastEvaluatedAt *time.Time `json:"last_evaluated_at,omitempty" jsonschema:"When the members were last checked against the whole account; missing until the first build ends."`
	CreatedAt       time.Time  `json:"created_at" jsonschema:"When the segment was created."`
	UpdatedAt       time.Time  `json:"updated_at" jsonschema:"When the segment was last saved."`
}

func segmentSummaryFrom(s *responses.Segment) segmentSummary {
	return segmentSummary{
		ID:              s.ID.String(),
		Name:            s.Name,
		Description:     s.Description,
		State:           s.State,
		FailureReason:   deref(s.FailureReason),
		MemberCount:     s.MemberCount,
		Timezone:        s.Timezone,
		LastEvaluatedAt: s.LastEvaluatedAt,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
	}
}

// segmentOutput is a segment with its rules.
type segmentOutput struct {
	segmentSummary
	Definition segmentDefinition `json:"definition" jsonschema:"The segment's rules."`
}

func segmentFrom(s *responses.Segment) segmentOutput {
	return segmentOutput{segmentSummary: segmentSummaryFrom(s), Definition: segmentDefinitionFrom(s.Definition)}
}

// segmentDetail is a segment as get_segment returns it.
type segmentDetail struct {
	segmentOutput
	MarketableCount *int `json:"marketable_count,omitempty" jsonschema:"Members a campaign to this segment alone would reach: enabled, not unsubscribed, not found invalid, and confirmed on at least one list or on no list at all. Missing while the segment is not ready, and when they cannot be counted in time."`
}

type segmentsOutput struct {
	Segments []segmentSummary `json:"segments" jsonschema:"This page's segments."`
	pageOutput
}

func segmentsFrom(resp *responses.PaginatedSegmentsResponse) segmentsOutput {
	out := segmentsOutput{Segments: []segmentSummary{}, pageOutput: contactPageOutputFrom(resp.Pagination)}
	for i := range resp.Data {
		out.Segments = append(out.Segments, segmentSummaryFrom(&resp.Data[i]))
	}
	return out
}

func listSegments(ctx context.Context, env *toolEnv, in pageInput) (segmentsOutput, error) {
	page, err := in.params()
	if err != nil {
		return segmentsOutput{}, err
	}
	resp, _, err := env.client.SegmentsAPI.GetSegments(ctx, env.accountID, requests.GetSegmentsParams{
		Limit: page.Limit,
		After: page.After,
	})
	if err != nil {
		return segmentsOutput{}, err
	}
	return segmentsFrom(resp), nil
}

type segmentIDInput struct {
	SegmentID string `json:"segment_id" jsonschema:"Segment ID."`
}

func getSegment(ctx context.Context, env *toolEnv, in segmentIDInput) (segmentDetail, error) {
	id, err := parseID("segment_id", in.SegmentID)
	if err != nil {
		return segmentDetail{}, err
	}
	s, _, err := env.client.SegmentsAPI.GetSegment(ctx, env.accountID, id)
	if err != nil {
		return segmentDetail{}, err
	}
	return segmentDetail{segmentOutput: segmentFrom(&s.Segment), MarketableCount: s.MarketableCount}, nil
}

type createSegmentInput struct {
	Name        string            `json:"name" jsonschema:"Segment name."`
	Description string            `json:"description,omitempty" jsonschema:"Description."`
	Definition  segmentDefinition `json:"definition" jsonschema:"The rules."`
	Timezone    string            `json:"timezone,omitempty" jsonschema:"IANA time zone, such as Europe/Berlin, that relative times and days are read in. Default UTC."`
}

func createSegment(ctx context.Context, env *toolEnv, in createSegmentInput) (segmentOutput, error) {
	definition, err := in.Definition.request()
	if err != nil {
		return segmentOutput{}, err
	}
	s, _, err := env.client.SegmentsAPI.CreateSegment(ctx, env.accountID, requests.CreateSegmentRequest{
		Name:        in.Name,
		Description: optionalString(in.Description),
		Definition:  definition,
		Timezone:    optionalString(strings.TrimSpace(in.Timezone)),
	})
	if err != nil {
		return segmentOutput{}, err
	}
	return segmentFrom(s), nil
}

type updateSegmentInput struct {
	SegmentID   string             `json:"segment_id" jsonschema:"Segment ID."`
	Name        string             `json:"name,omitempty" jsonschema:"New name."`
	Description *string            `json:"description,omitempty" jsonschema:"New description. An empty value clears it."`
	Definition  *segmentDefinition `json:"definition,omitempty" jsonschema:"New rules, replacing the segment's rules."`
	Timezone    string             `json:"timezone,omitempty" jsonschema:"New IANA time zone, such as Europe/Berlin."`
}

func updateSegment(ctx context.Context, env *toolEnv, in updateSegmentInput) (segmentOutput, error) {
	id, err := parseID("segment_id", in.SegmentID)
	if err != nil {
		return segmentOutput{}, err
	}
	req := requests.UpdateSegmentRequest{
		Name:        optionalString(in.Name),
		Description: in.Description,
		Timezone:    optionalString(strings.TrimSpace(in.Timezone)),
	}
	if in.Definition != nil {
		definition, err := in.Definition.request()
		if err != nil {
			return segmentOutput{}, err
		}
		req.Definition = &definition
	}
	s, _, err := env.client.SegmentsAPI.UpdateSegment(ctx, env.accountID, id, req)
	if err != nil {
		return segmentOutput{}, err
	}
	return segmentFrom(s), nil
}

func deleteSegment(ctx context.Context, env *toolEnv, in segmentIDInput) (confirmationOutput, error) {
	id, err := parseID("segment_id", in.SegmentID)
	if err != nil {
		return confirmationOutput{}, err
	}
	resp, _, err := env.client.SegmentsAPI.DeleteSegment(ctx, env.accountID, id)
	if err != nil {
		return confirmationOutput{}, err
	}
	return confirmationOutput{Message: resp.Message}, nil
}

type previewSegmentInput struct {
	Definition segmentDefinition `json:"definition" jsonschema:"The rules to try."`
	Timezone   string            `json:"timezone,omitempty" jsonschema:"IANA time zone, such as Europe/Berlin, that relative times and days are read in. Default UTC."`
}

type segmentPreviewContact struct {
	ContactID string `json:"contact_id" jsonschema:"Contact ID."`
	Email     string `json:"email" jsonschema:"Email address."`
}

type previewSegmentOutput struct {
	Count    *int                    `json:"count,omitempty" jsonschema:"Contacts the rules match now, whatever their consent. Missing when they cannot be counted in time."`
	Contacts []segmentPreviewContact `json:"contacts" jsonschema:"Up to 10 of the matching contacts, in no order."`
}

func previewSegment(ctx context.Context, env *toolEnv, in previewSegmentInput) (previewSegmentOutput, error) {
	definition, err := in.Definition.request()
	if err != nil {
		return previewSegmentOutput{}, err
	}
	resp, _, err := env.client.SegmentsAPI.PreviewSegment(ctx, env.accountID, requests.PreviewSegmentRequest{
		Definition: definition,
		Timezone:   optionalString(strings.TrimSpace(in.Timezone)),
	})
	if err != nil {
		return previewSegmentOutput{}, err
	}
	out := previewSegmentOutput{Count: resp.Count, Contacts: []segmentPreviewContact{}}
	for _, c := range resp.Contacts {
		out.Contacts = append(out.Contacts, segmentPreviewContact{ContactID: c.ContactID.String(), Email: c.Email})
	}
	return out, nil
}

type listSegmentContactsInput struct {
	pageInput
	SegmentID string `json:"segment_id" jsonschema:"Segment ID."`
}

type segmentMember struct {
	ContactID string    `json:"contact_id" jsonschema:"Contact ID."`
	Email     string    `json:"email" jsonschema:"Email address."`
	EnteredAt time.Time `json:"entered_at" jsonschema:"When the contact became a member."`
}

type listSegmentContactsOutput struct {
	Contacts []segmentMember `json:"contacts" jsonschema:"This page's members."`
	pageOutput
}

func listSegmentContacts(ctx context.Context, env *toolEnv, in listSegmentContactsInput) (listSegmentContactsOutput, error) {
	id, err := parseID("segment_id", in.SegmentID)
	if err != nil {
		return listSegmentContactsOutput{}, err
	}
	page, err := in.params()
	if err != nil {
		return listSegmentContactsOutput{}, err
	}
	resp, _, err := env.client.SegmentsAPI.GetSegmentContacts(ctx, env.accountID, id, requests.GetSegmentContactsParams{
		Limit: page.Limit,
		After: page.After,
	})
	if err != nil {
		return listSegmentContactsOutput{}, err
	}
	out := listSegmentContactsOutput{Contacts: []segmentMember{}, pageOutput: contactPageOutputFrom(resp.Pagination)}
	for _, m := range resp.Data {
		out.Contacts = append(out.Contacts, segmentMember{ContactID: m.ContactID.String(), Email: m.Email, EnteredAt: m.EnteredAt})
	}
	return out, nil
}

type listContactSegmentsInput struct {
	pageInput
	Contact string `json:"contact" jsonschema:"The contact's ID or email address."`
}

func listContactSegments(ctx context.Context, env *toolEnv, in listContactSegmentsInput) (segmentsOutput, error) {
	ref, err := contactRef(in.Contact)
	if err != nil {
		return segmentsOutput{}, err
	}
	page, err := in.params()
	if err != nil {
		return segmentsOutput{}, err
	}
	resp, _, err := env.client.SegmentsAPI.GetContactSegments(ctx, env.accountID, ref, requests.GetContactSegmentsParams{
		Limit: page.Limit,
		After: page.After,
	})
	if err != nil {
		return segmentsOutput{}, err
	}
	return segmentsFrom(resp), nil
}

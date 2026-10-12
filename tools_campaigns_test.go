package ahasendmcp

import (
	"context"
	"net/http"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCampaignTools(t *testing.T) {
	campaign := "/campaigns/" + testCampaignID
	stats := map[string]any{
		"object": "campaign_stats", "campaign_id": testCampaignID, "recipients": 120, "delivered": 118,
		"opens": 60, "clicks": 12, "unsubscribes": 1, "complaints": 0,
		"report": map[string]any{
			"computed_at": fixtureTime, "recipients": 120, "openers": 50, "clickers": 10, "bounces": 2, "deferrals": 0,
			"by_status":                map[string]any{"Delivered": 118, "Bounced": 2},
			"by_bounce_classification": map[string]any{"invalid_recipient": 2},
		},
		"rates":          map[string]any{"open_rate": 42.37, "click_rate": 8.47, "bounce_rate": 1.67, "deferral_rate": nil},
		"recount_failed": false,
	}

	runToolCases(t, []toolCase{
		{
			tool: "list_campaigns", args: map[string]any{"status": "sent"},
			method: http.MethodGet, path: "/campaigns", wantQuery: map[string]string{"status": "sent", "limit": "20"},
			resp: page(campaignJSON()),
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, testCampaignID, field(t, out, "campaigns.0.id"))
				assert.Equal(t, "news@mail.example.com", field(t, out, "campaigns.0.from.email"))
				assert.Nil(t, field(t, out, "campaigns.0.list_ids"), "lists are not in list results")
			},
		},
		{
			tool: "get_campaign", args: map[string]any{"campaign_id": testCampaignID},
			method: http.MethodGet, path: campaign,
			resp: func() map[string]any {
				c := campaignJSON()
				c["ready"] = false
				c["issues"] = []map[string]any{{"code": "preheader_missing", "message": "Add a preview text."}}
				return c
			}(),
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, false, out["ready"])
				assert.Equal(t, "preheader_missing", field(t, out, "issues.0.code"))
				assert.Equal(t, testList2ID, field(t, out, "list_ids.1"))
				assert.Contains(t, field(t, out, "content.html"), "CAMPAIGN-HTML-CONTENT")
			},
		},
		{
			tool: "create_campaign",
			args: map[string]any{
				"name": "October news", "subject": "Our October news", "list_ids": []string{testListID},
				"from":        map[string]any{"email": "news@mail.example.com"},
				"template_id": testCampaignTemplateID,
			},
			method: http.MethodPost, path: "/campaigns", status: http.StatusCreated, resp: campaignJSON(),
			wantBody: map[string]any{
				"name": "October news", "subject": "Our October news", "list_ids": []any{testListID},
				"from": map[string]any{"email": "news@mail.example.com"}, "template_id": testCampaignTemplateID,
			},
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, true, out["ready"])
				assert.NotContains(t, out, "content", "writes do not echo the content")
			},
		},
		{
			tool:   "update_campaign",
			args:   map[string]any{"campaign_id": testCampaignID, "preheader": "New", "list_ids": []string{testList2ID}, "html": "<p>x</p>"},
			method: http.MethodPut, path: campaign, resp: campaignJSON(),
			wantBody: map[string]any{"preheader": "New", "list_ids": []any{testList2ID}, "content": map[string]any{"html": "<p>x</p>"}},
		},
		{
			tool: "delete_campaign", args: map[string]any{"campaign_id": testCampaignID},
			method: http.MethodDelete, path: campaign, resp: success("campaign deleted"),
		},
		{
			tool: "duplicate_campaign", args: map[string]any{"campaign_id": testCampaignID, "name": "Again"},
			method: http.MethodPost, path: campaign + "/duplicate", status: http.StatusCreated, resp: campaignJSON(),
			wantBody: map[string]any{"name": "Again"},
		},
		{
			tool: "send_campaign_test", args: map[string]any{"campaign_id": testCampaignID, "to": []string{"me@example.com"}},
			method: http.MethodPost, path: campaign + "/test",
			resp:     map[string]any{"object": "campaign_test", "sent_to": []string{"me@example.com"}, "failed": []any{}, "result_unknown": false},
			wantBody: map[string]any{"to": []any{"me@example.com"}},
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, []any{"me@example.com"}, out["sent_to"])
			},
		},
		{
			tool: "unschedule_campaign", args: map[string]any{"campaign_id": testCampaignID},
			method: http.MethodPost, path: campaign + "/unschedule", resp: campaignJSON(),
			check: func(t *testing.T, out map[string]any) { assert.Equal(t, "draft", out["status"]) },
		},
		{
			tool: "get_campaign_stats", args: map[string]any{"campaign_id": testCampaignID},
			method: http.MethodGet, path: campaign + "/stats", resp: stats,
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, float64(118), out["delivered"])
				assert.Equal(t, float64(2), field(t, out, "report.by_status.Bounced"))
				assert.Equal(t, 42.37, field(t, out, "rates.open_rate"))
			},
		},
		{
			tool:   "list_campaign_recipients",
			args:   map[string]any{"campaign_id": testCampaignID, "bounce_classification": "invalid_recipient"},
			method: http.MethodGet, path: campaign + "/recipients",
			wantQuery: map[string]string{"bounce_classification": "invalid_recipient"},
			noQuery:   []string{"status"},
			resp: page(map[string]any{
				"object": "campaign_recipient", "contact_id": testContactID, "email": "ann@example.org",
				"list_name": "Newsletter", "status": "Bounced", "bounce_classification": "invalid_recipient",
				"bounce_reason": "no such user", "bounce_message": "550 5.1.1", "attempts": 1,
				"created_at": fixtureTime, "updated_at": fixtureTime,
			}),
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, "550 5.1.1", field(t, out, "recipients.0.bounce_message"))
			},
		},
	})
}

func TestListCampaignRecipientsNeedsExactlyOneFilter(t *testing.T) {
	f := newFakeAPI(t)
	cs := connect(t, testOptions(t, f))
	requireToolError(t, callTool(t, cs, "list_campaign_recipients", map[string]any{"campaign_id": testCampaignID}))
	requireToolError(t, callTool(t, cs, "list_campaign_recipients", map[string]any{
		"campaign_id": testCampaignID, "status": "Delivered", "bounce_classification": "invalid_recipient",
	}))
	assert.Empty(t, f.calls())
}

func TestSendCampaignTestNamesTheIssuesOfACampaignNotReady(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("POST "+apiPath("/campaigns/"+testCampaignID+"/test"), http.StatusBadRequest, map[string]any{
		"message": "campaign is not ready to send",
		"issues":  []map[string]any{{"code": "subject_missing", "message": "Add a subject."}},
	})
	cs := connect(t, testOptions(t, f))
	text := requireToolError(t, callTool(t, cs, "send_campaign_test", map[string]any{"campaign_id": testCampaignID, "to": []string{"me@example.com"}}))
	assert.Contains(t, text, "campaign is not ready to send")
	assert.Contains(t, text, "Add a subject")
	assert.Contains(t, text, "subject_missing")
}

func TestSendCampaignTestAndUnscheduleNeedNoConfirmation(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("POST "+apiPath("/campaigns/"+testCampaignID+"/test"), http.StatusOK,
		map[string]any{"object": "campaign_test", "sent_to": []string{"me@example.com"}, "failed": []any{}})
	f.handle("POST "+apiPath("/campaigns/"+testCampaignID+"/unschedule"), http.StatusOK, campaignJSON())
	cs := connectWith(t, testOptions(t, f), &mcp.ClientOptions{
		ElicitationHandler: func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			t.Error("the user must not be asked")
			return &mcp.ElicitResult{Action: "decline"}, nil
		},
	}, "")

	decode[map[string]any](t, callTool(t, cs, "send_campaign_test", map[string]any{"campaign_id": testCampaignID, "to": []string{"me@example.com"}}))
	decode[map[string]any](t, callTool(t, cs, "unschedule_campaign", map[string]any{"campaign_id": testCampaignID}))

	calls := f.calls()
	require.Len(t, calls, 2)
	assert.Equal(t, apiPath("/campaigns/"+testCampaignID+"/test"), calls[0].Path)
	assert.Equal(t, apiPath("/campaigns/"+testCampaignID+"/unschedule"), calls[1].Path)
}

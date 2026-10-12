package ahasendmcp

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// An empty value clears a field, sent as ahasend-go sends a clear for it: a
// null for the campaign fields and every address, "" for the others, which
// the API clears on. An omitted field is not sent.
func TestUpdateToolsClearEmptyFieldsAndLeaveOmittedOnes(t *testing.T) {
	template := "/templates/" + testTemplateID
	campaign := "/campaigns/" + testCampaignID
	contact := "/contacts/" + testContactID
	list := "/lists/" + testListID

	runToolCases(t, []toolCase{
		{
			tool: "update_template",
			args: map[string]any{
				"template_id": testTemplateID, "subject": "", "preheader": "",
				"from": map[string]any{"email": ""}, "reply_to": "",
			},
			method: http.MethodPut, path: template, resp: templateJSON(),
			wantBody: map[string]any{"subject": "", "preheader": "", "from": nil, "reply_to": nil},
		},
		{
			tool: "update_campaign",
			args: map[string]any{
				"campaign_id": testCampaignID, "subject": "", "preheader": "", "reply_to": "",
				"from": map[string]any{"email": ""},
			},
			method: http.MethodPut, path: campaign, resp: campaignJSON(),
			wantBody: map[string]any{"subject": nil, "preheader": nil, "reply_to": nil, "from": nil},
		},
		{
			tool:   "update_contact",
			args:   map[string]any{"contact": testContactID, "first_name": "", "last_name": "", "status_reason": ""},
			method: http.MethodPut, path: contact, resp: contactJSON(),
			wantBody: map[string]any{"first_name": "", "last_name": "", "status_reason": ""},
		},
		{
			tool: "update_list", args: map[string]any{"list_id": testListID, "description": ""},
			method: http.MethodPut, path: list, resp: listJSON(testListID, "Newsletter", 1),
			wantBody: map[string]any{"description": ""},
		},
	})

	t.Run("omitted fields are not sent", func(t *testing.T) {
		runToolCases(t, []toolCase{
			{
				tool: "update_template", args: map[string]any{"template_id": testTemplateID, "name": "Welcome"},
				method: http.MethodPut, path: template, resp: templateJSON(),
				wantBody: map[string]any{"name": "Welcome"},
			},
			{
				tool: "update_campaign", args: map[string]any{"campaign_id": testCampaignID, "name": "News"},
				method: http.MethodPut, path: campaign, resp: campaignJSON(),
				wantBody: map[string]any{"name": "News"},
			},
			{
				tool: "update_contact", args: map[string]any{"contact": testContactID, "status": "disabled"},
				method: http.MethodPut, path: contact, resp: contactJSON(),
				wantBody: map[string]any{"status": "disabled"},
			},
			{
				tool: "update_list", args: map[string]any{"list_id": testListID, "name": "News"},
				method: http.MethodPut, path: list, resp: listJSON(testListID, "News", 1),
				wantBody: map[string]any{"name": "News"},
			},
		})
	})

	t.Run("a null is the same as an omitted field", func(t *testing.T) {
		runToolCases(t, []toolCase{{
			tool: "update_list", args: map[string]any{"list_id": testListID, "name": "News", "description": nil},
			method: http.MethodPut, path: list, resp: listJSON(testListID, "News", 1),
			wantBody: map[string]any{"name": "News"},
		}})
	})
}

func TestClearableFieldsSayAnEmptyValueClearsThem(t *testing.T) {
	cs := connect(t, testOptions(t, newFakeAPI(t)))
	want := map[string][]string{
		"update_template": {"subject", "preheader", "reply_to"},
		"update_campaign": {"subject", "preheader", "reply_to"},
		"update_contact":  {"first_name", "last_name", "status_reason"},
		"update_list":     {"description"},
	}
	schemas := inputProperties(t, cs)
	for tool, fields := range want {
		for _, f := range fields {
			assert.Contains(t, schemas[tool][f], "An empty value clears it.", "%s.%s", tool, f)
		}
	}
}

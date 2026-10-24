package ahasendmcp

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManagementTools(t *testing.T) {
	const webhookSecret = "aha-whsec-SIGNING-SECRET"
	const routeSecret = "aha-route-SIGNING-SECRET"
	webhook := "/webhooks/" + testWebhookID
	route := "/routes/" + testRouteID

	runToolCases(t, []toolCase{
		{
			tool: "get_account", args: map[string]any{},
			method: http.MethodGet, path: "",
			resp: map[string]any{
				"object": "account", "id": testAccountID.String(), "created_at": fixtureTime, "updated_at": fixtureTime,
				"name": "Example", "website": "https://example.com", "track_opens": true,
				"message_data_retention": 30, "owner_id": testContactID,
			},
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, "Example", out["name"])
				assert.Equal(t, float64(30), out["message_data_retention_days"])
			},
		},
		{
			tool: "list_webhooks", args: map[string]any{"enabled": true},
			method: http.MethodGet, path: "/webhooks", wantQuery: map[string]string{"enabled": "true", "limit": "20"},
			resp: page(webhookJSON(webhookSecret)),
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, testWebhookID, field(t, out, "webhooks.0.id"))
				assert.Equal(t, true, field(t, out, "webhooks.0.on_bounced"))
				assert.Nil(t, field(t, out, "webhooks.0.secret"))
			},
		},
		{
			tool: "get_webhook", args: map[string]any{"webhook_id": testWebhookID},
			method: http.MethodGet, path: webhook, resp: webhookJSON(webhookSecret),
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, "scoped", out["scope"])
				assert.NotContains(t, out, "secret")
			},
		},
		{
			tool: "create_webhook",
			args: map[string]any{
				"name": "App events", "url": "https://app.example.com/hooks",
				"on_delivered": true, "on_bounced": true, "domains": []string{"mail.example.com"},
			},
			method: http.MethodPost, path: "/webhooks", status: http.StatusCreated, resp: webhookJSON(webhookSecret),
			wantBody: map[string]any{
				"name": "App events", "url": "https://app.example.com/hooks",
				"on_reception": false, "on_delivered": true, "on_transient_error": false, "on_failed": false,
				"on_bounced": true, "on_suppressed": false, "on_opened": false, "on_clicked": false,
				"on_suppression_created": false, "on_dns_error": false,
				// ahasend-go sends every event setting; this server does not offer the newer ones yet.
				"on_unsubscribed": false, "on_complained": false, "on_suppression_deleted": false,
				"on_contact_created": false, "on_contact_updated": false, "on_contact_deleted": false,
				"on_contact_added_to_list": false, "on_contact_removed_from_list": false,
				"on_contact_subscribed": false, "on_contact_unsubscribed": false, "on_contact_complained": false,
				"scope": "scoped", "domains": []any{"mail.example.com"},
			},
			check: func(t *testing.T, out map[string]any) { assert.Equal(t, webhookSecret, out["secret"]) },
		},
		{
			tool:   "update_webhook",
			args:   map[string]any{"webhook_id": testWebhookID, "enabled": false, "on_opened": true, "all_domains": true},
			method: http.MethodPut, path: webhook, resp: webhookJSON(""),
			wantBody: map[string]any{"enabled": false, "on_opened": true, "scope": "global"},
		},
		{
			tool: "delete_webhook", args: map[string]any{"webhook_id": testWebhookID},
			method: http.MethodDelete, path: webhook, resp: success("webhook deleted"),
		},
		{
			tool: "list_routes", args: map[string]any{"domain": "Mail.Example.com"},
			method: http.MethodGet, path: "/routes", wantQuery: map[string]string{"domain": "mail.example.com"},
			resp: page(routeJSON(routeSecret)),
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, "support@mail.example.com", field(t, out, "routes.0.recipient"))
				assert.Nil(t, field(t, out, "routes.0.secret"))
			},
		},
		{
			tool: "get_route", args: map[string]any{"route_id": testRouteID},
			method: http.MethodGet, path: route, resp: routeJSON(routeSecret),
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, true, out["strip_replies"])
				assert.NotContains(t, out, "secret")
			},
		},
		{
			tool: "create_route",
			args: map[string]any{
				"name": "Support", "url": "https://app.example.com/inbound", "recipient": "support@mail.example.com",
				"attachments": true, "strip_replies": true,
			},
			method: http.MethodPost, path: "/routes", status: http.StatusCreated, resp: routeJSON(routeSecret),
			wantBody: map[string]any{
				"name": "Support", "url": "https://app.example.com/inbound", "recipient": "support@mail.example.com",
				"attachments": true, "headers": false, "group_by_message_id": false, "strip_replies": true,
			},
			check: func(t *testing.T, out map[string]any) { assert.Equal(t, routeSecret, out["secret"]) },
		},
		{
			tool:   "update_route",
			args:   map[string]any{"route_id": testRouteID, "url": "https://app.example.com/v2", "headers": true},
			method: http.MethodPut, path: route, resp: routeJSON(""),
			wantBody: map[string]any{"url": "https://app.example.com/v2", "headers": true},
		},
		{
			tool: "delete_route", args: map[string]any{"route_id": testRouteID},
			method: http.MethodDelete, path: route, resp: success("route deleted"),
		},
		{
			tool: "list_suppressions", args: map[string]any{"email": "Ann@Example.org", "domain": "mail.example.com"},
			method: http.MethodGet, path: "/suppressions",
			wantQuery: map[string]string{"email": "ann@example.org", "domain": "mail.example.com"},
			resp:      page(suppressionJSON()),
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, "asked to stop", field(t, out, "suppressions.0.reason"))
			},
		},
		{
			tool:   "add_suppression",
			args:   map[string]any{"email": "Ann@Example.org", "expires_at": "2027-01-01T00:00:00Z", "reason": "asked to stop"},
			method: http.MethodPost, path: "/suppressions", status: http.StatusCreated,
			resp:     map[string]any{"object": "list", "data": []map[string]any{suppressionJSON()}},
			wantBody: map[string]any{"email": "ann@example.org", "expires_at": "2027-01-01T00:00:00Z", "reason": "asked to stop"},
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, "mail.example.com", field(t, out, "suppressions.0.domain"))
			},
		},
		{
			tool: "remove_suppression", args: map[string]any{"email": "Ann@Example.org"},
			method: http.MethodDelete, path: "/suppressions", wantQuery: map[string]string{"email": "ann@example.org"},
			noQuery: []string{"domain"},
			resp:    success("suppression deleted"),
		},
		{
			tool: "list_smtp_credentials", args: map[string]any{},
			method: http.MethodGet, path: "/smtp-credentials", wantQuery: map[string]string{"limit": "20"},
			resp: page(smtpCredentialJSON()),
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, "smtp-user", field(t, out, "smtp_credentials.0.username"))
			},
		},
		{
			tool: "get_smtp_credential", args: map[string]any{"smtp_credential_id": testSMTPCredentialID},
			method: http.MethodGet, path: "/smtp-credentials/" + testSMTPCredentialID, resp: smtpCredentialJSON(),
			check: func(t *testing.T, out map[string]any) { assert.Equal(t, "global", out["scope"]) },
		},
		{
			tool: "delete_smtp_credential", args: map[string]any{"smtp_credential_id": testSMTPCredentialID},
			method: http.MethodDelete, path: "/smtp-credentials/" + testSMTPCredentialID, resp: success("deleted"),
		},
		{
			tool: "list_sub_accounts", args: map[string]any{"cursor": "s-1"},
			method: http.MethodGet, path: "/sub-accounts", wantQuery: map[string]string{"after": "s-1"},
			resp: page(subAccountJSON()),
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, float64(5000), field(t, out, "sub_accounts.0.monthly_credit"))
			},
		},
		{
			tool: "get_sub_account", args: map[string]any{"sub_account_id": testSubAccountID},
			method: http.MethodGet, path: "/sub-accounts/" + testSubAccountID, resp: subAccountJSON(),
			check: func(t *testing.T, out map[string]any) { assert.Equal(t, "Client A", out["name"]) },
		},
		{
			tool:   "create_sub_account",
			args:   map[string]any{"name": "Client A", "website": "https://client-a.example", "monthly_credit": 5000},
			method: http.MethodPost, path: "/sub-accounts", status: http.StatusCreated, resp: subAccountJSON(),
			wantBody: map[string]any{"name": "Client A", "website": "https://client-a.example", "monthly_credit": float64(5000)},
			check:    func(t *testing.T, out map[string]any) { assert.Equal(t, testSubAccountID, out["id"]) },
		},
		{
			tool:   "update_sub_account",
			args:   map[string]any{"sub_account_id": testSubAccountID, "monthly_credit": 0},
			method: http.MethodPut, path: "/sub-accounts/" + testSubAccountID, resp: subAccountJSON(),
			wantBody: map[string]any{"monthly_credit": float64(0)},
		},
		{
			tool: "get_sub_account_usage", args: map[string]any{},
			method: http.MethodGet, path: "/sub-accounts/usage",
			resp: map[string]any{
				"billing_period": map[string]any{"start": "2026-10-01T00:00:00Z", "end": "2026-11-01T00:00:00Z"},
				"currency":       "EUR", "allocation_method": "proportional",
				"parent":               map[string]any{"account_id": testAccountID.String(), "name": "Example", "reception_count": 100, "allocated_cost": 1.5},
				"sub_accounts":         []map[string]any{{"account_id": testSubAccountID, "name": "Client A", "reception_count": 300, "allocated_cost": 4.5}},
				"removed_sub_accounts": map[string]any{"reception_count": 0, "allocated_cost": 0},
				"total":                map[string]any{"reception_count": 400, "allocated_cost": 6},
			},
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, "Client A", field(t, out, "sub_accounts.0.name"))
				assert.Equal(t, float64(400), field(t, out, "total.reception_count"))
			},
		},
	})
}

func TestCreateWebhookAndRouteReturnTheSigningSecret(t *testing.T) {
	const secret = "aha-SIGNING-SECRET-0123"
	tests := []struct {
		tool string
		path string
		args map[string]any
		body map[string]any
	}{
		{"create_webhook", "/webhooks", map[string]any{"name": "Hooks", "url": "https://app.example.com/hooks", "on_delivered": true}, webhookJSON(secret)},
		{"create_route", "/routes", map[string]any{"name": "Inbound", "url": "https://app.example.com/in", "recipient": "*@mail.example.com"}, routeJSON(secret)},
	}
	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			f := newFakeAPI(t)
			f.handle("POST "+apiPath(tt.path), http.StatusCreated, tt.body)
			cs := connect(t, testOptions(t, f))
			out := decode[map[string]any](t, callTool(t, cs, tt.tool, tt.args))
			assert.Equal(t, secret, out["secret"])
		})
	}
}

func TestCreateWebhookWithoutDomainsIsGlobal(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("POST "+apiPath("/webhooks"), http.StatusCreated, webhookJSON("s"))
	cs := connect(t, testOptions(t, f))
	decode[map[string]any](t, callTool(t, cs, "create_webhook", map[string]any{"name": "Hooks", "url": "https://app.example.com/hooks"}))
	body := f.onlyCall(t).Body
	assert.Equal(t, "global", body["scope"])
	assert.NotContains(t, body, "domains")
}

func TestSMTPCredentialToolsNeverReturnThePassword(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("GET "+apiPath("/smtp-credentials"), http.StatusOK, page(smtpCredentialJSON()))
	f.handle("GET "+apiPath("/smtp-credentials/"+testSMTPCredentialID), http.StatusOK, smtpCredentialJSON())
	cs := connect(t, testOptions(t, f))

	for _, call := range []struct {
		tool string
		args map[string]any
	}{
		{"list_smtp_credentials", map[string]any{}},
		{"get_smtp_credential", map[string]any{"smtp_credential_id": testSMTPCredentialID}},
	} {
		res := callTool(t, cs, call.tool, call.args)
		require.False(t, res.IsError, resultText(res))
		raw, err := json.Marshal(res)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), "SMTP-PASSWORD-SECRET", call.tool)
	}
}

func TestUpdateWebhookRefusesDomainsWithAllDomains(t *testing.T) {
	f := newFakeAPI(t)
	cs := connect(t, testOptions(t, f))
	requireToolError(t, callTool(t, cs, "update_webhook", map[string]any{
		"webhook_id": testWebhookID, "domains": []string{"mail.example.com"}, "all_domains": true,
	}))
	assert.Empty(t, f.calls())
}

func TestRemoveSuppressionSendsTheDomainWhenGiven(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("DELETE "+apiPath("/suppressions"), http.StatusOK, success("suppression deleted"))
	cs := connect(t, testOptions(t, f))

	decode[map[string]any](t, callTool(t, cs, "remove_suppression", map[string]any{"email": "ann@example.org", "domain": "Mail.Example.com"}))

	req := f.onlyCall(t)
	assert.Equal(t, "ann@example.org", req.Query.Get("email"))
	assert.Equal(t, "mail.example.com", req.Query.Get("domain"))
}

func TestRemoveSuppressionRefusesAnEmptyEmail(t *testing.T) {
	f := newFakeAPI(t)
	rec := &fakeRecorder{}
	opts := testOptions(t, f)
	opts.Recorder = rec
	cs := connect(t, opts)

	text := requireToolError(t, callTool(t, cs, "remove_suppression", map[string]any{"email": "  "}))
	assert.Contains(t, text, "email")
	assert.Empty(t, f.calls())
	assert.Equal(t, ErrorKindInvalidInput, rec.only(t).ErrorKind)
}

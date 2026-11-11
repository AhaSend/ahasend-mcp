package ahasendmcp

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListDomains(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("GET "+apiPath("/domains"), http.StatusOK, map[string]any{
		"object":     "list",
		"data":       []map[string]any{domainJSON("mail.example.com")},
		"pagination": map[string]any{"has_more": true, "next_cursor": "dom-2"},
	})
	cs := connect(t, testOptions(t, f))

	res := callTool(t, cs, "list_domains", map[string]any{"dns_valid": false, "sending_type": "transactional"})
	out := decode[listDomainsOutput](t, res)

	req := f.onlyCall(t)
	assert.Equal(t, http.MethodGet, req.Method)
	assert.Equal(t, apiPath("/domains"), req.Path)
	assert.Equal(t, "false", req.Query.Get("dns_valid"))
	assert.Equal(t, "transactional", req.Query.Get("sending_type"))
	assert.Equal(t, "20", req.Query.Get("limit"))

	require.Len(t, out.Domains, 1)
	assert.Equal(t, "mail.example.com", out.Domains[0].Domain)
	assert.False(t, out.Domains[0].DNSValid)
	assert.Equal(t, "dom-2", out.NextCursor)
	assert.NotContains(t, resultText(res), "v=DKIM1", "no DNS records in the list")
}

func TestGetDomainReturnsDNSRecords(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("GET "+apiPath("/domains/{domain}"), http.StatusOK, domainJSON("mail.example.com"))
	cs := connect(t, testOptions(t, f))

	out := decode[domainDetail](t, callTool(t, cs, "get_domain", map[string]any{"domain": "mail.example.com"}))

	req := f.onlyCall(t)
	assert.Equal(t, http.MethodGet, req.Method)
	assert.Equal(t, apiPath("/domains/mail.example.com"), req.Path)
	require.Len(t, out.DNSRecords, 2)
	assert.Equal(t, dnsRecord{Type: "TXT", Host: "aha-sel._domainkey.mail.example.com", Content: "v=DKIM1; p=abc", Required: true}, out.DNSRecords[0])
}

func TestAddDomain(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("POST "+apiPath("/domains"), http.StatusCreated, domainJSON("mail.example.com"))
	cs := connect(t, testOptions(t, f))

	out := decode[domainDetail](t, callTool(t, cs, "add_domain", map[string]any{
		"domain":             "mail.example.com",
		"sending_type":       "marketing",
		"tracking_subdomain": "click",
	}))

	req := f.onlyCall(t)
	assert.Equal(t, http.MethodPost, req.Method)
	assert.Equal(t, apiPath("/domains"), req.Path)
	assert.Equal(t, map[string]any{
		"domain":             "mail.example.com",
		"sending_type":       "marketing",
		"tracking_subdomain": "click",
	}, req.Body)
	assert.Len(t, out.DNSRecords, 2, "the result holds the records to add")
}

func TestCheckDomainDNS(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("POST "+apiPath("/domains/{domain}/check-dns"), http.StatusOK, domainJSON("mail.example.com"))
	cs := connect(t, testOptions(t, f))

	out := decode[domainDetail](t, callTool(t, cs, "check_domain_dns", map[string]any{"domain": "mail.example.com"}))

	req := f.onlyCall(t)
	assert.Equal(t, http.MethodPost, req.Method)
	assert.Equal(t, apiPath("/domains/mail.example.com/check-dns"), req.Path)
	assert.Nil(t, req.Body)
	assert.Equal(t, "mail.example.com", out.Domain)
}

func TestUpdateDomain(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("PUT "+apiPath("/domains/{domain}"), http.StatusOK, domainJSON("mail.example.com"))
	cs := connect(t, testOptions(t, f))

	decode[domainDetail](t, callTool(t, cs, "update_domain", map[string]any{
		"domain":                      "mail.example.com",
		"return_path_subdomain":       "bounce",
		"dkim_rotation_interval_days": 90,
	}))

	req := f.onlyCall(t)
	assert.Equal(t, http.MethodPut, req.Method)
	assert.Equal(t, apiPath("/domains/mail.example.com"), req.Path)
	assert.Equal(t, map[string]any{
		"return_path_subdomain":       "bounce",
		"dkim_rotation_interval_days": float64(90),
	}, req.Body)
}

func TestDeleteDomain(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("DELETE "+apiPath("/domains/{domain}"), http.StatusOK, map[string]any{"message": "Domain deleted"})
	cs := connect(t, testOptions(t, f))

	out := decode[confirmationOutput](t, callTool(t, cs, "delete_domain", map[string]any{"domain": "mail.example.com"}))

	req := f.onlyCall(t)
	assert.Equal(t, http.MethodDelete, req.Method)
	assert.Equal(t, apiPath("/domains/mail.example.com"), req.Path)
	assert.Equal(t, "Domain deleted", out.Message)
}

func TestAddDomainSendsTheDKIMSelectorWhenGiven(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want map[string]any
	}{
		{"not given", map[string]any{}, map[string]any{"domain": "mail.example.com"}},
		{"empty", map[string]any{"dkim_selector": ""}, map[string]any{"domain": "mail.example.com"}},
		{"a value", map[string]any{"dkim_selector": "os"}, map[string]any{"domain": "mail.example.com", "dkim_selector": "os"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]any{"domain": "mail.example.com"}
			for k, v := range tt.args {
				args[k] = v
			}
			runToolCases(t, []toolCase{{
				tool: "add_domain", args: args,
				method: http.MethodPost, path: "/domains", status: http.StatusCreated, resp: domainJSON("mail.example.com"),
				wantBody: tt.want,
			}})
		})
	}
}

func TestUpdateDomainSetsOrClearsTheDKIMSelector(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want map[string]any
	}{
		{"not given", map[string]any{"tracking_subdomain": "click"}, map[string]any{"tracking_subdomain": "click"}},
		{"empty clears it", map[string]any{"dkim_selector": ""}, map[string]any{"dkim_selector": ""}},
		{"a value", map[string]any{"dkim_selector": "os"}, map[string]any{"dkim_selector": "os"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]any{"domain": "mail.example.com"}
			for k, v := range tt.args {
				args[k] = v
			}
			runToolCases(t, []toolCase{{
				tool: "update_domain", args: args,
				method: http.MethodPut, path: "/domains/mail.example.com", resp: domainJSON("mail.example.com"),
				wantBody: tt.want,
			}})
		})
	}
}

// domainStateJSON is a paused domain with a custom DKIM selector, a ready
// standby key and a DSN recipient.
func domainStateJSON() map[string]any {
	d := domainJSON("mail.example.com")
	d["updated_at"] = "2026-09-15T08:30:00Z"
	d["dkim_selector"] = "os"
	d["rotation_ready"] = true
	d["dsn_recipient"] = "dsn@example.com"
	d["paused"] = true
	d["paused_at"] = "2026-09-20T12:00:00Z"
	d["pause_reason"] = "bounce_rate"
	return d
}

func TestDomainOutputsHaveTheDomainState(t *testing.T) {
	runToolCases(t, []toolCase{
		{
			tool: "get_domain", args: map[string]any{"domain": "mail.example.com"},
			method: http.MethodGet, path: "/domains/mail.example.com", resp: domainStateJSON(),
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, "os", out["dkim_selector"])
				assert.Equal(t, true, out["rotation_ready"])
				assert.Equal(t, "dsn@example.com", out["dsn_recipient"])
				assert.Equal(t, "2026-09-20T12:00:00Z", out["paused_at"])
				assert.Equal(t, "2026-09-15T08:30:00Z", out["updated_at"])
			},
		},
		{
			tool: "list_domains", args: map[string]any{},
			method: http.MethodGet, path: "/domains", resp: page(domainStateJSON()),
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, "2026-09-20T12:00:00Z", field(t, out, "domains.0.paused_at"))
				assert.Equal(t, "2026-09-15T08:30:00Z", field(t, out, "domains.0.updated_at"))
				assert.NotContains(t, field(t, out, "domains.0"), "dkim_selector", "list results hold no settings")
			},
		},
	})
}

func TestDomainOutputLeavesOutUnsetState(t *testing.T) {
	runToolCases(t, []toolCase{{
		tool: "get_domain", args: map[string]any{"domain": "mail.example.com"},
		method: http.MethodGet, path: "/domains/mail.example.com", resp: domainJSON("mail.example.com"),
		check: func(t *testing.T, out map[string]any) {
			assert.NotContains(t, out, "dkim_selector")
			assert.NotContains(t, out, "dsn_recipient")
			assert.NotContains(t, out, "paused_at")
			assert.Equal(t, false, out["rotation_ready"])
		},
	}})
}

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

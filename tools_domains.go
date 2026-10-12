package ahasendmcp

import (
	"context"
	"strings"
	"time"

	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
)

func domainsTools() []tool {
	return []tool{
		newTool(ToolInfo{
			Name:           "list_domains",
			Title:          "List domains",
			Toolset:        ToolsetDomains,
			RequiredScopes: []string{"domains:read"},
			Annotations:    annRead(),
		}, "Lists the sending domains of this account with their DNS state, sending type and pause state, but "+
			"not their DNS records. Use it to see which domains can send; get_domain returns one domain with "+
			"its records. The result holds one page and a next_cursor for the page after it.",
			listDomains),
		newTool(ToolInfo{
			Name:           "get_domain",
			Title:          "Get domain",
			Toolset:        ToolsetDomains,
			RequiredScopes: []string{"domains:read"},
			Annotations:    annRead(),
		}, "Returns one sending domain with the DNS records to add at its DNS provider and whether each is in "+
			"place. Use it to set up a domain or find why it cannot send. dns_valid is true once every required "+
			"record is in place; check_domain_dns checks the records again.",
			getDomain),
		newTool(ToolInfo{
			Name:           "add_domain",
			Title:          "Add domain",
			Toolset:        ToolsetDomains,
			RequiredScopes: []string{"domains:write"},
			Annotations:    annWrite(),
		}, "Adds a sending domain to this account. Use it before sending from a new domain. The result holds "+
			"the DNS records to add at the domain's DNS provider; the domain can send once they are in place "+
			"and check_domain_dns reports dns_valid true.",
			addDomain),
		newTool(ToolInfo{
			Name:           "check_domain_dns",
			Title:          "Check domain DNS",
			Toolset:        ToolsetDomains,
			RequiredScopes: []string{"domains:write"},
			Annotations:    annWriteIdempotent(),
		}, "Checks a sending domain's DNS records now and returns the domain with each record's state. Use it "+
			"after adding records at the DNS provider. A check within 60 seconds of the last one returns the "+
			"last result; DNS changes can take time to show.",
			checkDomainDNS),
		newTool(ToolInfo{
			Name:           "update_domain",
			Title:          "Update domain",
			Toolset:        ToolsetDomains,
			RequiredScopes: []string{"domains:write"},
			Annotations:    annWrite(),
		}, "Changes a sending domain's settings: its custom tracking, return-path, subscription and media "+
			"subdomains, its sending type, and its DKIM rotation interval. Use it when the user wants one of these "+
			"changed. Only the fields given change. A new "+
			"subdomain needs its own DNS record; the result holds the domain with its records.",
			updateDomain),
		newTool(ToolInfo{
			Name:           "delete_domain",
			Title:          "Delete domain",
			Toolset:        ToolsetDomains,
			RequiredScopes: []string{"domains:delete"},
			Annotations:    annDestructive(),
		}, "Deletes a sending domain from this account; email can no longer be sent from it. Use it only when "+
			"the user asks to remove the domain. A paused domain "+
			"cannot be deleted. The result is AhaSend's confirmation.",
			deleteDomain),
	}
}

// dnsRecord is one DNS record a domain needs.
type dnsRecord struct {
	Type       string `json:"type" jsonschema:"Record type, such as TXT, CNAME or MX."`
	Host       string `json:"host" jsonschema:"Host name to add the record at."`
	Content    string `json:"content" jsonschema:"Record value."`
	Label      string `json:"label,omitempty" jsonschema:"What the record is for."`
	Required   bool   `json:"required" jsonschema:"True when the domain cannot send without it."`
	Propagated bool   `json:"propagated" jsonschema:"True when the record was found in DNS."`
}

func dnsRecordFrom(r responses.DNSRecord) dnsRecord {
	out := dnsRecord{Type: r.Type, Host: r.Host, Content: r.Content, Required: r.Required, Propagated: r.Propagated}
	if r.Label != nil {
		out.Label = *r.Label
	}
	return out
}

// domainSummary is a domain as list_domains shows it.
type domainSummary struct {
	ID             string     `json:"id" jsonschema:"Domain ID."`
	Domain         string     `json:"domain" jsonschema:"Domain name."`
	SendingType    string     `json:"sending_type,omitempty" jsonschema:"transactional or marketing."`
	DNSValid       bool       `json:"dns_valid" jsonschema:"True when every required DNS record is in place."`
	Paused         bool       `json:"paused" jsonschema:"True when sending from the domain is paused."`
	PauseReason    string     `json:"pause_reason,omitempty" jsonschema:"Why sending is paused."`
	CreatedAt      time.Time  `json:"created_at" jsonschema:"When the domain was added."`
	LastDNSCheckAt *time.Time `json:"last_dns_check_at,omitempty" jsonschema:"When the DNS records were last checked."`
}

func domainSummaryFrom(d *responses.Domain) domainSummary {
	s := domainSummary{
		ID:             d.ID.String(),
		Domain:         d.Domain,
		SendingType:    d.SendingType,
		DNSValid:       d.DNSValid,
		Paused:         d.Paused,
		CreatedAt:      d.CreatedAt,
		LastDNSCheckAt: d.LastDNSCheckAt,
	}
	if d.PauseReason != nil {
		s.PauseReason = *d.PauseReason
	}
	return s
}

// domainDetail is a domain with its DNS records and settings.
type domainDetail struct {
	domainSummary
	DNSRecords               []dnsRecord `json:"dns_records" jsonschema:"DNS records to add at the domain's DNS provider."`
	TrackingSubdomain        string      `json:"tracking_subdomain,omitempty" jsonschema:"Custom tracking subdomain."`
	ReturnPathSubdomain      string      `json:"return_path_subdomain,omitempty" jsonschema:"Custom return-path subdomain."`
	SubscriptionSubdomain    string      `json:"subscription_subdomain,omitempty" jsonschema:"Custom subscription management subdomain."`
	MediaSubdomain           string      `json:"media_subdomain,omitempty" jsonschema:"Custom media subdomain."`
	DKIMRotationIntervalDays int         `json:"dkim_rotation_interval_days,omitempty" jsonschema:"DKIM key rotation interval in days."`
}

func domainDetailFrom(d *responses.Domain) domainDetail {
	out := domainDetail{
		domainSummary: domainSummaryFrom(d),
		DNSRecords:    []dnsRecord{},
	}
	for _, r := range d.DNSRecords {
		out.DNSRecords = append(out.DNSRecords, dnsRecordFrom(r))
	}
	out.TrackingSubdomain = deref(d.TrackingSubdomain)
	out.ReturnPathSubdomain = deref(d.ReturnPathSubdomain)
	out.SubscriptionSubdomain = deref(d.SubscriptionSubdomain)
	out.MediaSubdomain = deref(d.MediaSubdomain)
	out.DKIMRotationIntervalDays = deref(d.DKIMRotationIntervalDays)
	return out
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

type listDomainsInput struct {
	pageInput
	DNSValid    *bool  `json:"dns_valid,omitempty" jsonschema:"true for domains that can send, false for domains still missing DNS records."`
	SendingType string `json:"sending_type,omitempty" jsonschema:"transactional or marketing."`
}

type listDomainsOutput struct {
	Domains []domainSummary `json:"domains" jsonschema:"This page's domains."`
	pageOutput
}

func listDomains(ctx context.Context, env *toolEnv, in listDomainsInput) (listDomainsOutput, error) {
	page, err := in.params()
	if err != nil {
		return listDomainsOutput{}, err
	}
	params := requests.GetDomainsParams{
		DNSValid:         in.DNSValid,
		SendingType:      optionalString(in.SendingType),
		PaginationParams: page,
	}
	resp, _, err := env.client.DomainsAPI.GetDomainsWithParams(ctx, env.accountID, params)
	if err != nil {
		return listDomainsOutput{}, err
	}
	out := listDomainsOutput{Domains: []domainSummary{}, pageOutput: pageOutputFrom(resp.Pagination)}
	for i := range resp.Data {
		out.Domains = append(out.Domains, domainSummaryFrom(&resp.Data[i]))
	}
	return out, nil
}

type domainInput struct {
	Domain string `json:"domain" jsonschema:"Domain name, such as mail.example.com."`
}

func (in domainInput) name() (string, error) {
	name := strings.TrimSpace(in.Domain)
	if name == "" {
		return "", invalidInput("domain is required.")
	}
	return name, nil
}

func getDomain(ctx context.Context, env *toolEnv, in domainInput) (domainDetail, error) {
	name, err := in.name()
	if err != nil {
		return domainDetail{}, err
	}
	d, _, err := env.client.DomainsAPI.GetDomain(ctx, env.accountID, name)
	if err != nil {
		return domainDetail{}, err
	}
	return domainDetailFrom(d), nil
}

// domainSettings are the domain settings add_domain and update_domain take.
type domainSettings struct {
	SendingType              string `json:"sending_type,omitempty" jsonschema:"transactional or marketing. When omitted, add_domain uses transactional and update_domain keeps the current value."`
	TrackingSubdomain        string `json:"tracking_subdomain,omitempty" jsonschema:"Custom tracking subdomain. When omitted, add_domain uses the default and update_domain keeps the current value."`
	ReturnPathSubdomain      string `json:"return_path_subdomain,omitempty" jsonschema:"Custom return-path subdomain. When omitted, add_domain uses the default and update_domain keeps the current value."`
	SubscriptionSubdomain    string `json:"subscription_subdomain,omitempty" jsonschema:"Custom subscription management subdomain. When omitted, add_domain uses the default and update_domain keeps the current value."`
	MediaSubdomain           string `json:"media_subdomain,omitempty" jsonschema:"Custom media subdomain. When omitted, add_domain uses the default and update_domain keeps the current value."`
	DKIMRotationIntervalDays int    `json:"dkim_rotation_interval_days,omitempty" jsonschema:"DKIM key rotation interval in days, for managed DNS domains on eligible plans."`
}

func optionalInt(v int) *int {
	if v == 0 {
		return nil
	}
	return ptr(v)
}

type addDomainInput struct {
	domainInput
	domainSettings
}

func addDomain(ctx context.Context, env *toolEnv, in addDomainInput) (domainDetail, error) {
	name, err := in.name()
	if err != nil {
		return domainDetail{}, err
	}
	req := requests.CreateDomainRequest{
		Domain:                   name,
		SendingType:              optionalString(in.SendingType),
		TrackingSubdomain:        optionalString(in.TrackingSubdomain),
		ReturnPathSubdomain:      optionalString(in.ReturnPathSubdomain),
		SubscriptionSubdomain:    optionalString(in.SubscriptionSubdomain),
		MediaSubdomain:           optionalString(in.MediaSubdomain),
		DKIMRotationIntervalDays: optionalInt(in.DKIMRotationIntervalDays),
	}
	d, _, err := env.client.DomainsAPI.CreateDomain(ctx, env.accountID, req)
	if err != nil {
		return domainDetail{}, err
	}
	return domainDetailFrom(d), nil
}

func checkDomainDNS(ctx context.Context, env *toolEnv, in domainInput) (domainDetail, error) {
	name, err := in.name()
	if err != nil {
		return domainDetail{}, err
	}
	d, _, err := env.client.DomainsAPI.CheckDomainDNS(ctx, env.accountID, name)
	if err != nil {
		return domainDetail{}, err
	}
	return domainDetailFrom(d), nil
}

type updateDomainInput struct {
	domainInput
	domainSettings
}

func updateDomain(ctx context.Context, env *toolEnv, in updateDomainInput) (domainDetail, error) {
	name, err := in.name()
	if err != nil {
		return domainDetail{}, err
	}
	req := requests.UpdateDomainRequest{
		SendingType:              optionalString(in.SendingType),
		TrackingSubdomain:        optionalString(in.TrackingSubdomain),
		ReturnPathSubdomain:      optionalString(in.ReturnPathSubdomain),
		SubscriptionSubdomain:    optionalString(in.SubscriptionSubdomain),
		MediaSubdomain:           optionalString(in.MediaSubdomain),
		DKIMRotationIntervalDays: optionalInt(in.DKIMRotationIntervalDays),
	}
	d, _, err := env.client.DomainsAPI.UpdateDomain(ctx, env.accountID, name, req)
	if err != nil {
		return domainDetail{}, err
	}
	return domainDetailFrom(d), nil
}

func deleteDomain(ctx context.Context, env *toolEnv, in domainInput) (confirmationOutput, error) {
	name, err := in.name()
	if err != nil {
		return confirmationOutput{}, err
	}
	resp, _, err := env.client.DomainsAPI.DeleteDomain(ctx, env.accountID, name)
	if err != nil {
		return confirmationOutput{}, err
	}
	return confirmationOutput{Message: resp.Message}, nil
}

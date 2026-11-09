package ahasendmcp

import (
	"context"
	"strings"
	"time"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
)

func managementTools() []tool {
	return []tool{
		newTool(ToolInfo{
			Name:           "get_account",
			Title:          "Get account",
			Toolset:        ToolsetManagement,
			RequiredScopes: []string{"accounts:read"},
			Annotations:    annRead(),
		}, "Returns this AhaSend account: its name, website, parent account (for a sub-account), tracking "+
			"defaults, recipient checks and how many days message data is kept. Use it to answer questions about "+
			"the account's settings; the user changes them in the AhaSend dashboard.",
			getAccount),
		newTool(ToolInfo{
			Name:           "list_webhooks",
			Title:          "List webhooks",
			Toolset:        ToolsetManagement,
			RequiredScopes: []string{"webhooks:read"},
			Annotations:    annRead(),
		}, "Lists the webhooks of this account with their URL, the events each sends, the domains and origins it covers "+
			"and its success and error counts since it was created, but not their signing secrets. Use it to find a webhook or "+
			"see why events are not arriving. A connection limited to some domains sees only their webhooks. "+
			"The result holds one page and a next_cursor for the page after it.",
			listWebhooks),
		newTool(ToolInfo{
			Name:           "get_webhook",
			Title:          "Get webhook",
			Toolset:        ToolsetManagement,
			RequiredScopes: []string{"webhooks:read"},
			Annotations:    annRead(),
		}, "Returns one webhook with its URL, events, domains, origins and success and error counts since it was "+
			"created, but not its "+
			"signing secret, which AhaSend returns only when the webhook is created. Use it to check a webhook's "+
			"settings or why its posts fail.",
			getWebhook),
		newTool(ToolInfo{
			Name:           "create_webhook",
			Title:          "Create webhook",
			Toolset:        ToolsetManagement,
			RequiredScopes: []string{"webhooks:write"},
			Annotations:    annWrite(),
		}, "Creates a webhook that posts the chosen email events to a URL, for every sending domain or only "+
			"the domains given. A webhook for every domain needs the webhooks:write:all scope; a connection "+
			"limited to some domains must list them in domains. Use it when the user's app needs delivery, bounce, open, click, unsubscribe, "+
			"complaint, suppression or contact events. Contact events need the contacts:read scope and the account's marketing features; "+
			"the added_to_list, removed_from_list, subscribed, unsubscribed and complained ones also need lists:read. The result holds the webhook's signing secret, which the app needs to verify the posts; "+
			"AhaSend returns it only in this result.",
			createWebhook),
		newTool(ToolInfo{
			Name:           "update_webhook",
			Title:          "Update webhook",
			Toolset:        ToolsetManagement,
			RequiredScopes: []string{"webhooks:write"},
			Annotations:    annWrite(),
		}, "Changes a webhook's name, URL, enabled state, events, origins or domains. Use it to point a webhook at a new "+
			"URL, turn it off or change what it sends. Only the fields given change. While the webhook is enabled with contact events on, "+
			"every update needs the scopes of those events; without them, only turning those events off or disabling the webhook works. "+
			"all_domains needs the webhooks:write:all scope; a connection limited to some domains can only "+
			"list them in domains. "+
			"The signing secret cannot change. The result is the changed webhook.",
			updateWebhook),
		newTool(ToolInfo{
			Name:           "delete_webhook",
			Title:          "Delete webhook",
			Toolset:        ToolsetManagement,
			RequiredScopes: []string{"webhooks:delete"},
			Annotations:    annDestructive(),
		}, "Deletes a webhook; its URL gets no more events. Use it only when the user asks to remove the "+
			"webhook. The result is AhaSend's confirmation.",
			deleteWebhook),
		newTool(ToolInfo{
			Name:           "list_routes",
			Title:          "List routes",
			Toolset:        ToolsetManagement,
			RequiredScopes: []string{"routes:read"},
			Annotations:    annRead(),
		}, "Lists the inbound routes of this account: each forwards email received at matching addresses to a "+
			"URL. The result shows each route's address pattern, URL and success and error counts since it was "+
			"created, but "+
			"not its signing secret. Use it to find the route for an inbound address or see why received email "+
			"is not forwarded. A connection limited to some domains must give domain. The result holds "+
			"one page and a next_cursor for the page after it.",
			listRoutes),
		newTool(ToolInfo{
			Name:           "get_route",
			Title:          "Get route",
			Toolset:        ToolsetManagement,
			RequiredScopes: []string{"routes:read"},
			Annotations:    annRead(),
		}, "Returns one inbound route with its address pattern, URL, options and success and error counts "+
			"since it was created, but not its signing secret, which AhaSend returns only when the route is created. "+
			"Use it to check a route's settings or why its posts fail.",
			getRoute),
		newTool(ToolInfo{
			Name:           "create_route",
			Title:          "Create route",
			Toolset:        ToolsetManagement,
			RequiredScopes: []string{"routes:write"},
			Annotations:    annWrite(),
		}, "Creates an inbound route that forwards email received at matching addresses of one of this "+
			"account's domains to a URL. Use it when the user's app should receive email. The result holds the "+
			"route's signing secret, which the app needs to verify the posts; AhaSend returns it only in this "+
			"result.",
			createRoute),
		newTool(ToolInfo{
			Name:           "update_route",
			Title:          "Update route",
			Toolset:        ToolsetManagement,
			RequiredScopes: []string{"routes:write"},
			Annotations:    annWrite(),
		}, "Changes an inbound route's name, URL, address pattern, options or enabled state. Use it to point "+
			"a route at a new URL, turn it off or change what it posts. Only the fields "+
			"given change. The result is the changed route.",
			updateRoute),
		newTool(ToolInfo{
			Name:           "delete_route",
			Title:          "Delete route",
			Toolset:        ToolsetManagement,
			RequiredScopes: []string{"routes:delete"},
			Annotations:    annDestructive(),
		}, "Deletes an inbound route; email to its addresses is no longer forwarded. Use it only when the user "+
			"asks to remove the route. The result is AhaSend's confirmation.",
			deleteRoute),
		newTool(ToolInfo{
			Name:           "list_suppressions",
			Title:          "List suppressions",
			Toolset:        ToolsetManagement,
			RequiredScopes: []string{"suppressions:read"},
			Annotations:    annRead(),
		}, "Lists suppressed recipient addresses, which AhaSend does not send to until each suppression "+
			"expires, with the reason and the sending domain each applies to. Use it to check why an address "+
			"gets no email; filter by exact email, domain or creation time. The result holds one page and a "+
			"next_cursor for the page after it.",
			listSuppressions),
		newTool(ToolInfo{
			Name:           "add_suppression",
			Title:          "Add suppression",
			Toolset:        ToolsetManagement,
			RequiredScopes: []string{"suppressions:write"},
			Annotations:    annWrite(),
		}, "Suppresses a recipient address until expires_at, so AhaSend sends it no email, from one sending "+
			"domain or, without domain, from each domain of the account. Use it when someone must not get email. "+
			"The result lists the suppressions made, one per domain.",
			addSuppression),
		newTool(ToolInfo{
			Name:           "remove_suppression",
			Title:          "Remove suppression",
			Toolset:        ToolsetManagement,
			RequiredScopes: []string{"suppressions:delete"},
			Annotations:    annDestructive(),
		}, "Removes the suppressions of one recipient address, for one sending domain or, without domain, for "+
			"every domain, so AhaSend sends to it again. It also removes protected suppressions, which record "+
			"the recipient's own unsubscribe or spam report; list_suppressions shows which are protected. Use it "+
			"when the recipient wants the email again. Sending to an address suppressed for bounces or complaints "+
			"can hurt delivery. The result is AhaSend's confirmation.",
			removeSuppression),
		newTool(ToolInfo{
			Name:           "list_smtp_credentials",
			Title:          "List SMTP credentials",
			Toolset:        ToolsetManagement,
			RequiredScopes: []string{"smtp-credentials:read"},
			Annotations:    annRead(),
		}, "Lists the SMTP credentials of this account with their username, sandbox flag and the domains they "+
			"may send from, never their passwords. Use it to find a credential. A connection limited to some "+
			"domains sees only their credentials. The result holds one page and a next_cursor for the page "+
			"after it.",
			listSMTPCredentials),
		newTool(ToolInfo{
			Name:           "get_smtp_credential",
			Title:          "Get SMTP credential",
			Toolset:        ToolsetManagement,
			RequiredScopes: []string{"smtp-credentials:read"},
			Annotations:    annRead(),
		}, "Returns one SMTP credential with its username, sandbox flag and domains, never its password. Use it "+
			"to check which domains an app's SMTP login may send from.",
			getSMTPCredential),
		newTool(ToolInfo{
			Name:           "delete_smtp_credential",
			Title:          "Delete SMTP credential",
			Toolset:        ToolsetManagement,
			RequiredScopes: []string{"smtp-credentials:delete"},
			Annotations:    annDestructive(),
		}, "Deletes an SMTP credential; apps that use it can no longer send. Use it only when the user asks to "+
			"remove the credential, such as after a leak. The result is AhaSend's confirmation.",
			deleteSMTPCredential),
		newTool(ToolInfo{
			Name:           "list_sub_accounts",
			Title:          "List sub-accounts",
			Toolset:        ToolsetManagement,
			RequiredScopes: []string{"sub-accounts:read"},
			Annotations:    annRead(),
		}, "Lists the sub-accounts of this account with their status, monthly credit cap and domain and member "+
			"counts. Use it to find a sub-account. The result holds one page and a next_cursor for the page "+
			"after it.",
			listSubAccounts),
		newTool(ToolInfo{
			Name:           "get_sub_account",
			Title:          "Get sub-account",
			Toolset:        ToolsetManagement,
			RequiredScopes: []string{"sub-accounts:read"},
			Annotations:    annRead(),
		}, "Returns one sub-account of this account with its status, monthly credit cap, domain and member "+
			"counts and last activity. Use it to check one sub-account's state.",
			getSubAccount),
		newTool(ToolInfo{
			Name:           "create_sub_account",
			Title:          "Create sub-account",
			Toolset:        ToolsetManagement,
			RequiredScopes: []string{"sub-accounts:write"},
			Annotations:    annWrite(),
		}, "Creates a sub-account under this account, such as one for a client of the user. Use it when the "+
			"user asks for a new sub-account. The result is the new sub-account with its ID.",
			createSubAccount),
		newTool(ToolInfo{
			Name:           "update_sub_account",
			Title:          "Update sub-account",
			Toolset:        ToolsetManagement,
			RequiredScopes: []string{"sub-accounts:write"},
			Annotations:    annWrite(),
		}, "Changes a sub-account's name, website or monthly credit cap. Use it when the user wants these "+
			"changed. Only the fields given change, and at "+
			"least one must be given. The result is the changed sub-account.",
			updateSubAccount),
		newTool(ToolInfo{
			Name:           "get_sub_account_usage",
			Title:          "Get sub-account usage",
			Toolset:        ToolsetManagement,
			RequiredScopes: []string{"sub-accounts:usage"},
			Annotations:    annRead(),
		}, "Returns this billing period's email count and share of the cost for this account and each of its "+
			"sub-accounts that are not deleted (suspended ones included), with the totals and how the cost was shared. Use it to answer which "+
			"sub-account sends how much.",
			getSubAccountUsage),
	}
}

// Account.

type accountOutput struct {
	ID                       string    `json:"id" jsonschema:"Account ID."`
	Name                     string    `json:"name" jsonschema:"Account name."`
	ParentAccountID          string    `json:"parent_account_id,omitempty" jsonschema:"The parent account, for a sub-account."`
	Website                  string    `json:"website,omitempty" jsonschema:"Website."`
	TrackOpens               *bool     `json:"track_opens,omitempty" jsonschema:"Default for open tracking."`
	TrackClicks              *bool     `json:"track_clicks,omitempty" jsonschema:"Default for click tracking."`
	RejectBadRecipients      *bool     `json:"reject_bad_recipients,omitempty" jsonschema:"Whether bad recipient addresses are rejected."`
	RejectMistypedRecipients *bool     `json:"reject_mistyped_recipients,omitempty" jsonschema:"Whether mistyped recipient addresses are rejected."`
	MessageMetadataRetention *int32    `json:"message_metadata_retention_days,omitempty" jsonschema:"Days message metadata is kept."`
	MessageDataRetention     *int32    `json:"message_data_retention_days,omitempty" jsonschema:"Days message content is kept."`
	CreatedAt                time.Time `json:"created_at" jsonschema:"When the account was created."`
}

func getAccount(ctx context.Context, env *toolEnv, _ struct{}) (accountOutput, error) {
	a, _, err := env.client.AccountsAPI.GetAccount(ctx, env.accountID)
	if err != nil {
		return accountOutput{}, err
	}
	out := accountOutput{
		ID:                       a.ID.String(),
		Name:                     a.Name,
		Website:                  deref(a.Website),
		TrackOpens:               a.TrackOpens,
		TrackClicks:              a.TrackClicks,
		RejectBadRecipients:      a.RejectBadRecipients,
		RejectMistypedRecipients: a.RejectMistypedRecipients,
		MessageMetadataRetention: a.MessageMetadataRetention,
		MessageDataRetention:     a.MessageDataRetention,
		CreatedAt:                a.CreatedAt,
	}
	if a.ParentAccountID != nil {
		out.ParentAccountID = a.ParentAccountID.String()
	}
	return out, nil
}

// Webhooks.

// webhookEvents are the events a webhook sends.
type webhookEvents struct {
	OnReception              bool `json:"on_reception" jsonschema:"AhaSend accepted a message and queued it for delivery."`
	OnDelivered              bool `json:"on_delivered" jsonschema:"Message delivered."`
	OnTransientError         bool `json:"on_transient_error" jsonschema:"Delivery deferred; it is retried later."`
	OnFailed                 bool `json:"on_failed" jsonschema:"Delivery failed after repeated attempts; no more retries."`
	OnBounced                bool `json:"on_bounced" jsonschema:"The receiving server sent a bounce, also possible after delivery."`
	OnSuppressed             bool `json:"on_suppressed" jsonschema:"Message not sent to a suppressed recipient."`
	OnOpened                 bool `json:"on_opened" jsonschema:"Message opened."`
	OnClicked                bool `json:"on_clicked" jsonschema:"A tracked link was clicked."`
	OnSuppressionCreated     bool `json:"on_suppression_created" jsonschema:"suppression.created: a recipient was suppressed, such as after a bounce, a spam report, a transactional unsubscribe or an API call."`
	OnDNSError               bool `json:"on_dns_error" jsonschema:"A DNS problem was found on a sending domain."`
	OnUnsubscribed           bool `json:"on_unsubscribed" jsonschema:"message.unsubscribed: a recipient unsubscribed through a campaign email's link or one-click unsubscribe."`
	OnComplained             bool `json:"on_complained" jsonschema:"message.complained: a mailbox provider reported a campaign email as spam."`
	OnSuppressionDeleted     bool `json:"on_suppression_deleted" jsonschema:"suppression.deleted: a suppression was deleted through the API or the dashboard, or by AhaSend; deleting all suppressions at once and the removal of expired ones send none."`
	OnContactCreated         bool `json:"on_contact_created" jsonschema:"contact.created: a contact was created through the API or the dashboard; CSV imports send no contact events. Contact events need the contacts:read scope."`
	OnContactUpdated         bool `json:"on_contact_updated" jsonschema:"contact.updated: a contact's email, name, status or attributes changed. Needs contacts:read."`
	OnContactDeleted         bool `json:"on_contact_deleted" jsonschema:"contact.deleted: a contact was deleted. Needs contacts:read."`
	OnContactAddedToList     bool `json:"on_contact_added_to_list" jsonschema:"contact.added_to_list: a contact was added to a list; CSV imports send none. Needs contacts:read and lists:read."`
	OnContactRemovedFromList bool `json:"on_contact_removed_from_list" jsonschema:"contact.removed_from_list: a contact was removed from a list. Needs contacts:read and lists:read."`
	OnContactSubscribed      bool `json:"on_contact_subscribed" jsonschema:"contact.subscribed: a list membership became confirmed, or the contact's unsubscribe from all lists was cleared. Needs contacts:read and lists:read."`
	OnContactUnsubscribed    bool `json:"on_contact_unsubscribed" jsonschema:"contact.unsubscribed: a list membership became unsubscribed, or the contact unsubscribed from all lists. Needs contacts:read and lists:read."`
	OnContactComplained      bool `json:"on_contact_complained" jsonschema:"contact.complained: a contact reported a campaign email as spam. Needs contacts:read and lists:read."`
}

// webhookEventChanges are event changes; a missing event keeps its setting.
type webhookEventChanges struct {
	OnReception              *bool `json:"on_reception,omitempty" jsonschema:"AhaSend accepted a message and queued it for delivery."`
	OnDelivered              *bool `json:"on_delivered,omitempty" jsonschema:"Message delivered."`
	OnTransientError         *bool `json:"on_transient_error,omitempty" jsonschema:"Delivery deferred; it is retried later."`
	OnFailed                 *bool `json:"on_failed,omitempty" jsonschema:"Delivery failed after repeated attempts; no more retries."`
	OnBounced                *bool `json:"on_bounced,omitempty" jsonschema:"The receiving server sent a bounce, also possible after delivery."`
	OnSuppressed             *bool `json:"on_suppressed,omitempty" jsonschema:"Message not sent to a suppressed recipient."`
	OnOpened                 *bool `json:"on_opened,omitempty" jsonschema:"Message opened."`
	OnClicked                *bool `json:"on_clicked,omitempty" jsonschema:"A tracked link was clicked."`
	OnSuppressionCreated     *bool `json:"on_suppression_created,omitempty" jsonschema:"suppression.created: a recipient was suppressed, such as after a bounce, a spam report, a transactional unsubscribe or an API call."`
	OnDNSError               *bool `json:"on_dns_error,omitempty" jsonschema:"A DNS problem was found on a sending domain."`
	OnUnsubscribed           *bool `json:"on_unsubscribed,omitempty" jsonschema:"message.unsubscribed: a recipient unsubscribed through a campaign email's link or one-click unsubscribe."`
	OnComplained             *bool `json:"on_complained,omitempty" jsonschema:"message.complained: a mailbox provider reported a campaign email as spam."`
	OnSuppressionDeleted     *bool `json:"on_suppression_deleted,omitempty" jsonschema:"suppression.deleted: a suppression was deleted through the API or the dashboard, or by AhaSend; deleting all suppressions at once and the removal of expired ones send none."`
	OnContactCreated         *bool `json:"on_contact_created,omitempty" jsonschema:"contact.created: a contact was created through the API or the dashboard; CSV imports send no contact events. Contact events need the contacts:read scope."`
	OnContactUpdated         *bool `json:"on_contact_updated,omitempty" jsonschema:"contact.updated: a contact's email, name, status or attributes changed. Needs contacts:read."`
	OnContactDeleted         *bool `json:"on_contact_deleted,omitempty" jsonschema:"contact.deleted: a contact was deleted. Needs contacts:read."`
	OnContactAddedToList     *bool `json:"on_contact_added_to_list,omitempty" jsonschema:"contact.added_to_list: a contact was added to a list; CSV imports send none. Needs contacts:read and lists:read."`
	OnContactRemovedFromList *bool `json:"on_contact_removed_from_list,omitempty" jsonschema:"contact.removed_from_list: a contact was removed from a list. Needs contacts:read and lists:read."`
	OnContactSubscribed      *bool `json:"on_contact_subscribed,omitempty" jsonschema:"contact.subscribed: a list membership became confirmed, or the contact's unsubscribe from all lists was cleared. Needs contacts:read and lists:read."`
	OnContactUnsubscribed    *bool `json:"on_contact_unsubscribed,omitempty" jsonschema:"contact.unsubscribed: a list membership became unsubscribed, or the contact unsubscribed from all lists. Needs contacts:read and lists:read."`
	OnContactComplained      *bool `json:"on_contact_complained,omitempty" jsonschema:"contact.complained: a contact reported a campaign email as spam. Needs contacts:read and lists:read."`
}

type webhookOutput struct {
	ID      string `json:"id" jsonschema:"Webhook ID."`
	Name    string `json:"name" jsonschema:"Webhook name."`
	URL     string `json:"url" jsonschema:"URL the events are posted to."`
	Enabled bool   `json:"enabled" jsonschema:"True when events are posted."`
	webhookEvents
	Scope                  string     `json:"scope" jsonschema:"global for every sending domain, scoped for the domains listed."`
	Domains                []string   `json:"domains,omitempty" jsonschema:"The domains a scoped webhook covers."`
	Origins                []string   `json:"origins" jsonschema:"Origins whose contact and suppression events it gets: recipient, api, dashboard or system; null for every origin."`
	SuccessCount           uint64     `json:"success_count" jsonschema:"Posts that succeeded."`
	ErrorCount             uint64     `json:"error_count" jsonschema:"Posts that failed."`
	ErrorsSinceLastSuccess int        `json:"errors_since_last_success" jsonschema:"Failed posts since the last success."`
	LastRequestAt          *time.Time `json:"last_request_at,omitempty" jsonschema:"When the last post was made."`
	CreatedAt              time.Time  `json:"created_at" jsonschema:"When the webhook was created."`
	Secret                 string     `json:"secret,omitempty" jsonschema:"Signing secret; returned only when the webhook is created."`
}

func webhookFrom(w *responses.Webhook) webhookOutput {
	return webhookOutput{
		ID:      w.ID.String(),
		Name:    w.Name,
		URL:     w.URL,
		Enabled: w.Enabled,
		webhookEvents: webhookEvents{
			OnReception:              w.OnReception,
			OnDelivered:              w.OnDelivered,
			OnTransientError:         w.OnTransientError,
			OnFailed:                 w.OnFailed,
			OnBounced:                w.OnBounced,
			OnSuppressed:             w.OnSuppressed,
			OnOpened:                 w.OnOpened,
			OnClicked:                w.OnClicked,
			OnSuppressionCreated:     w.OnSuppressionCreated,
			OnDNSError:               w.OnDNSError,
			OnUnsubscribed:           w.OnUnsubscribed,
			OnComplained:             w.OnComplained,
			OnSuppressionDeleted:     w.OnSuppressionDeleted,
			OnContactCreated:         w.OnContactCreated,
			OnContactUpdated:         w.OnContactUpdated,
			OnContactDeleted:         w.OnContactDeleted,
			OnContactAddedToList:     w.OnContactAddedToList,
			OnContactRemovedFromList: w.OnContactRemovedFromList,
			OnContactSubscribed:      w.OnContactSubscribed,
			OnContactUnsubscribed:    w.OnContactUnsubscribed,
			OnContactComplained:      w.OnContactComplained,
		},
		Origins:                w.Origins,
		Scope:                  w.Scope,
		Domains:                w.Domains,
		SuccessCount:           w.SuccessCount,
		ErrorCount:             w.ErrorCount,
		ErrorsSinceLastSuccess: w.ErrorsSinceLastSuccess,
		LastRequestAt:          w.LastRequestAt,
		CreatedAt:              w.CreatedAt,
		Secret:                 w.Secret,
	}
}

type listWebhooksInput struct {
	pageInput
	Enabled *bool `json:"enabled,omitempty" jsonschema:"true for enabled webhooks only, false for disabled ones."`
}

type listWebhooksOutput struct {
	Webhooks []webhookOutput `json:"webhooks" jsonschema:"This page's webhooks."`
	pageOutput
}

func listWebhooks(ctx context.Context, env *toolEnv, in listWebhooksInput) (listWebhooksOutput, error) {
	page, err := in.params()
	if err != nil {
		return listWebhooksOutput{}, err
	}
	resp, _, err := env.client.WebhooksAPI.GetWebhooks(ctx, env.accountID, api.GetWebhooksParams{
		Enabled:          in.Enabled,
		PaginationParams: page,
	})
	if err != nil {
		return listWebhooksOutput{}, err
	}
	out := listWebhooksOutput{Webhooks: []webhookOutput{}, pageOutput: pageOutputFrom(resp.Pagination)}
	for i := range resp.Data {
		w := webhookFrom(&resp.Data[i])
		w.Secret = ""
		out.Webhooks = append(out.Webhooks, w)
	}
	return out, nil
}

type webhookIDInput struct {
	WebhookID string `json:"webhook_id" jsonschema:"Webhook ID."`
}

func getWebhook(ctx context.Context, env *toolEnv, in webhookIDInput) (webhookOutput, error) {
	id, err := parseID("webhook_id", in.WebhookID)
	if err != nil {
		return webhookOutput{}, err
	}
	w, _, err := env.client.WebhooksAPI.GetWebhook(ctx, env.accountID, id)
	if err != nil {
		return webhookOutput{}, err
	}
	out := webhookFrom(w)
	out.Secret = ""
	return out, nil
}

type createWebhookInput struct {
	Name    string   `json:"name" jsonschema:"Webhook name."`
	URL     string   `json:"url" jsonschema:"URL to post the events to."`
	Enabled *bool    `json:"enabled,omitempty" jsonschema:"false to create it disabled. Default true."`
	Domains []string `json:"domains,omitempty" jsonschema:"Sending domains whose events it posts; omit for every domain, which needs the webhooks:write:all scope."`
	Origins []string `json:"origins,omitempty" jsonschema:"Origins whose contact and suppression events it gets: recipient (an unsubscribe link, a one-click unsubscribe or a spam complaint), api (writes through the AhaSend API, including this connection's tools), dashboard (the AhaSend dashboard), system (AhaSend itself, such as after a bounce). Message and DNS events ignore it. Omit for every origin. Leave out api when the app writes contacts back through the API, or each write comes back as an event."`
	webhookEventChanges
}

func createWebhook(ctx context.Context, env *toolEnv, in createWebhookInput) (webhookOutput, error) {
	if err := checkWebhookOrigins(in.Origins, "leave it out for every origin."); err != nil {
		return webhookOutput{}, err
	}
	e := in.webhookEventChanges
	req := requests.CreateWebhookRequest{
		Name:                     in.Name,
		URL:                      in.URL,
		Enabled:                  in.Enabled,
		OnReception:              deref(e.OnReception),
		OnDelivered:              deref(e.OnDelivered),
		OnTransientError:         deref(e.OnTransientError),
		OnFailed:                 deref(e.OnFailed),
		OnBounced:                deref(e.OnBounced),
		OnSuppressed:             deref(e.OnSuppressed),
		OnOpened:                 deref(e.OnOpened),
		OnClicked:                deref(e.OnClicked),
		OnSuppressionCreated:     deref(e.OnSuppressionCreated),
		OnDnsError:               deref(e.OnDNSError),
		OnUnsubscribed:           deref(e.OnUnsubscribed),
		OnComplained:             deref(e.OnComplained),
		OnSuppressionDeleted:     deref(e.OnSuppressionDeleted),
		OnContactCreated:         deref(e.OnContactCreated),
		OnContactUpdated:         deref(e.OnContactUpdated),
		OnContactDeleted:         deref(e.OnContactDeleted),
		OnContactAddedToList:     deref(e.OnContactAddedToList),
		OnContactRemovedFromList: deref(e.OnContactRemovedFromList),
		OnContactSubscribed:      deref(e.OnContactSubscribed),
		OnContactUnsubscribed:    deref(e.OnContactUnsubscribed),
		OnContactComplained:      deref(e.OnContactComplained),
		Origins:                  in.Origins,
		Scope:                    "global",
	}
	if len(in.Domains) > 0 {
		req.Scope = "scoped"
		req.Domains = &in.Domains
	}
	w, _, err := env.client.WebhooksAPI.CreateWebhook(ctx, env.accountID, req)
	if err != nil {
		return webhookOutput{}, err
	}
	return webhookFrom(w), nil
}

type updateWebhookInput struct {
	WebhookID  string   `json:"webhook_id" jsonschema:"Webhook ID."`
	Name       string   `json:"name,omitempty" jsonschema:"New name."`
	URL        string   `json:"url,omitempty" jsonschema:"New URL."`
	Enabled    *bool    `json:"enabled,omitempty" jsonschema:"true to enable, false to disable."`
	Domains    []string `json:"domains,omitempty" jsonschema:"Limit the webhook to these sending domains. Not with all_domains."`
	AllDomains bool     `json:"all_domains,omitempty" jsonschema:"True to post events of every sending domain; needs the webhooks:write:all scope. Not with domains."`
	Origins    []string `json:"origins,omitempty" jsonschema:"Replaces the origins whose contact and suppression events it gets: recipient (an unsubscribe link, a one-click unsubscribe or a spam complaint), api (writes through the AhaSend API, including this connection's tools), dashboard (the AhaSend dashboard), system (AhaSend itself, such as after a bounce). Message and DNS events ignore it. Omit to keep them; give all four for every origin."`
	webhookEventChanges
}

func updateWebhook(ctx context.Context, env *toolEnv, in updateWebhookInput) (webhookOutput, error) {
	id, err := parseID("webhook_id", in.WebhookID)
	if err != nil {
		return webhookOutput{}, err
	}
	if in.AllDomains && len(in.Domains) > 0 {
		return webhookOutput{}, invalidInput("Give domains or all_domains, not both.")
	}
	if err := checkWebhookOrigins(in.Origins, "give all four for every origin, or leave it out to keep the current ones."); err != nil {
		return webhookOutput{}, err
	}
	e := in.webhookEventChanges
	req := requests.UpdateWebhookRequest{
		Name:                     optionalString(in.Name),
		URL:                      optionalString(in.URL),
		Enabled:                  in.Enabled,
		OnReception:              e.OnReception,
		OnDelivered:              e.OnDelivered,
		OnTransientError:         e.OnTransientError,
		OnFailed:                 e.OnFailed,
		OnBounced:                e.OnBounced,
		OnSuppressed:             e.OnSuppressed,
		OnOpened:                 e.OnOpened,
		OnClicked:                e.OnClicked,
		OnSuppressionCreated:     e.OnSuppressionCreated,
		OnDnsError:               e.OnDNSError,
		OnUnsubscribed:           e.OnUnsubscribed,
		OnComplained:             e.OnComplained,
		OnSuppressionDeleted:     e.OnSuppressionDeleted,
		OnContactCreated:         e.OnContactCreated,
		OnContactUpdated:         e.OnContactUpdated,
		OnContactDeleted:         e.OnContactDeleted,
		OnContactAddedToList:     e.OnContactAddedToList,
		OnContactRemovedFromList: e.OnContactRemovedFromList,
		OnContactSubscribed:      e.OnContactSubscribed,
		OnContactUnsubscribed:    e.OnContactUnsubscribed,
		OnContactComplained:      e.OnContactComplained,
		Origins:                  in.Origins,
	}
	switch {
	case in.AllDomains:
		req.Scope = ptr("global")
	case len(in.Domains) > 0:
		req.Scope = ptr("scoped")
		req.Domains = &in.Domains
	}
	w, _, err := env.client.WebhooksAPI.UpdateWebhook(ctx, env.accountID, id, req)
	if err != nil {
		return webhookOutput{}, err
	}
	out := webhookFrom(w)
	out.Secret = ""
	return out, nil
}

// checkWebhookOrigins refuses an origins list that the API refuses: an empty
// one, which would turn the contact and suppression events off without
// saying so, or one with an unknown origin. whenEmpty tells the caller what to
// send instead.
func checkWebhookOrigins(origins []string, whenEmpty string) error {
	if origins == nil {
		return nil
	}
	if len(origins) == 0 {
		return invalidInput("origins needs at least one origin; %s", whenEmpty)
	}
	for _, o := range origins {
		switch o {
		case common.WebhookOriginRecipient, common.WebhookOriginAPI, common.WebhookOriginDashboard, common.WebhookOriginSystem:
		default:
			return invalidInput("Unknown origin %q; use recipient, api, dashboard or system.", o)
		}
	}
	return nil
}

func deleteWebhook(ctx context.Context, env *toolEnv, in webhookIDInput) (confirmationOutput, error) {
	id, err := parseID("webhook_id", in.WebhookID)
	if err != nil {
		return confirmationOutput{}, err
	}
	resp, _, err := env.client.WebhooksAPI.DeleteWebhook(ctx, env.accountID, id)
	if err != nil {
		return confirmationOutput{}, err
	}
	return confirmationOutput{Message: resp.Message}, nil
}

// Routes.

// routeOptions are what a route includes in its posts.
type routeOptions struct {
	Attachments      bool `json:"attachments" jsonschema:"True to include attachments in the posts."`
	Headers          bool `json:"headers" jsonschema:"True to include the email headers in the posts."`
	GroupByMessageID bool `json:"group_by_message_id" jsonschema:"True to post a message once per Message-ID, even when several of its recipients match the route."`
	StripReplies     bool `json:"strip_replies" jsonschema:"True to add the reply text without the quoted history, as reply_from_plain_body; the body itself is unchanged."`
}

type routeOutput struct {
	ID        string `json:"id" jsonschema:"Route ID."`
	Name      string `json:"name" jsonschema:"Route name."`
	URL       string `json:"url" jsonschema:"URL the received email is posted to."`
	Recipient string `json:"recipient" jsonschema:"Address pattern the route takes email for."`
	Enabled   bool   `json:"enabled" jsonschema:"True when email is forwarded."`
	routeOptions
	SuccessCount           uint64     `json:"success_count" jsonschema:"Posts that succeeded."`
	ErrorCount             uint64     `json:"error_count" jsonschema:"Posts that failed."`
	ErrorsSinceLastSuccess int        `json:"errors_since_last_success" jsonschema:"Failed posts since the last success."`
	LastRequestAt          *time.Time `json:"last_request_at,omitempty" jsonschema:"When the last post was made."`
	CreatedAt              time.Time  `json:"created_at" jsonschema:"When the route was created."`
	Secret                 string     `json:"secret,omitempty" jsonschema:"Signing secret; returned only when the route is created."`
}

func routeFrom(r *responses.Route) routeOutput {
	return routeOutput{
		ID:        r.ID.String(),
		Name:      r.Name,
		URL:       r.URL,
		Recipient: r.Recipient,
		Enabled:   r.Enabled,
		routeOptions: routeOptions{
			Attachments:      r.Attachments,
			Headers:          r.Headers,
			GroupByMessageID: r.GroupByMessageID,
			StripReplies:     r.StripReplies,
		},
		SuccessCount:           r.SuccessCount,
		ErrorCount:             r.ErrorCount,
		ErrorsSinceLastSuccess: r.ErrorsSinceLastSuccess,
		LastRequestAt:          r.LastRequestAt,
		CreatedAt:              r.CreatedAt,
		Secret:                 r.Secret,
	}
}

type listRoutesInput struct {
	pageInput
	Domain string `json:"domain,omitempty" jsonschema:"Only routes for this domain. Required when the connection may read routes of some domains only."`
}

type listRoutesOutput struct {
	Routes []routeOutput `json:"routes" jsonschema:"This page's routes."`
	pageOutput
}

func listRoutes(ctx context.Context, env *toolEnv, in listRoutesInput) (listRoutesOutput, error) {
	page, err := in.params()
	if err != nil {
		return listRoutesOutput{}, err
	}
	resp, _, err := env.client.RoutesAPI.GetRoutesWithParams(ctx, env.accountID, requests.GetRoutesParams{
		Domain:           optionalString(strings.ToLower(strings.TrimSpace(in.Domain))),
		PaginationParams: page,
	})
	if err != nil {
		return listRoutesOutput{}, err
	}
	out := listRoutesOutput{Routes: []routeOutput{}, pageOutput: pageOutputFrom(resp.Pagination)}
	for i := range resp.Data {
		r := routeFrom(&resp.Data[i])
		r.Secret = ""
		out.Routes = append(out.Routes, r)
	}
	return out, nil
}

type routeIDInput struct {
	RouteID string `json:"route_id" jsonschema:"Route ID."`
}

func getRoute(ctx context.Context, env *toolEnv, in routeIDInput) (routeOutput, error) {
	id, err := parseID("route_id", in.RouteID)
	if err != nil {
		return routeOutput{}, err
	}
	r, _, err := env.client.RoutesAPI.GetRoute(ctx, env.accountID, id)
	if err != nil {
		return routeOutput{}, err
	}
	out := routeFrom(r)
	out.Secret = ""
	return out, nil
}

type createRouteInput struct {
	Name             string `json:"name" jsonschema:"Route name."`
	URL              string `json:"url" jsonschema:"URL to post the received email to."`
	Recipient        string `json:"recipient" jsonschema:"Address to take email for, on a domain of this account, such as support@example.com; * in the part before the @ matches any text."`
	Enabled          *bool  `json:"enabled,omitempty" jsonschema:"false to create it disabled. Default true."`
	Attachments      bool   `json:"attachments,omitempty" jsonschema:"True to include attachments in the posts."`
	Headers          bool   `json:"headers,omitempty" jsonschema:"True to include the email headers in the posts."`
	GroupByMessageID bool   `json:"group_by_message_id,omitempty" jsonschema:"True to post a message once per Message-ID, even when several of its recipients match the route."`
	StripReplies     bool   `json:"strip_replies,omitempty" jsonschema:"True to add the reply text without the quoted history, as reply_from_plain_body; the body itself is unchanged."`
}

func createRoute(ctx context.Context, env *toolEnv, in createRouteInput) (routeOutput, error) {
	r, _, err := env.client.RoutesAPI.CreateRoute(ctx, env.accountID, requests.CreateRouteRequest{
		Name:             in.Name,
		URL:              in.URL,
		Recipient:        in.Recipient,
		Attachments:      in.Attachments,
		Headers:          in.Headers,
		GroupByMessageId: in.GroupByMessageID,
		StripReplies:     in.StripReplies,
		Enabled:          in.Enabled,
	})
	if err != nil {
		return routeOutput{}, err
	}
	return routeFrom(r), nil
}

type updateRouteInput struct {
	RouteID          string `json:"route_id" jsonschema:"Route ID."`
	Name             string `json:"name,omitempty" jsonschema:"New name."`
	URL              string `json:"url,omitempty" jsonschema:"New URL."`
	Recipient        string `json:"recipient,omitempty" jsonschema:"New address pattern."`
	Enabled          *bool  `json:"enabled,omitempty" jsonschema:"true to enable, false to disable."`
	Attachments      *bool  `json:"attachments,omitempty" jsonschema:"Include attachments in the posts."`
	Headers          *bool  `json:"headers,omitempty" jsonschema:"Include the email headers in the posts."`
	GroupByMessageID *bool  `json:"group_by_message_id,omitempty" jsonschema:"Post a message once per Message-ID, even when several of its recipients match the route."`
	StripReplies     *bool  `json:"strip_replies,omitempty" jsonschema:"Add the reply text without the quoted history, as reply_from_plain_body; the body itself is unchanged."`
}

func updateRoute(ctx context.Context, env *toolEnv, in updateRouteInput) (routeOutput, error) {
	id, err := parseID("route_id", in.RouteID)
	if err != nil {
		return routeOutput{}, err
	}
	r, _, err := env.client.RoutesAPI.UpdateRoute(ctx, env.accountID, id, requests.UpdateRouteRequest{
		Name:             optionalString(in.Name),
		URL:              optionalString(in.URL),
		Recipient:        optionalString(in.Recipient),
		Attachments:      in.Attachments,
		Headers:          in.Headers,
		GroupByMessageId: in.GroupByMessageID,
		StripReplies:     in.StripReplies,
		Enabled:          in.Enabled,
	})
	if err != nil {
		return routeOutput{}, err
	}
	out := routeFrom(r)
	out.Secret = ""
	return out, nil
}

func deleteRoute(ctx context.Context, env *toolEnv, in routeIDInput) (confirmationOutput, error) {
	id, err := parseID("route_id", in.RouteID)
	if err != nil {
		return confirmationOutput{}, err
	}
	resp, _, err := env.client.RoutesAPI.DeleteRoute(ctx, env.accountID, id)
	if err != nil {
		return confirmationOutput{}, err
	}
	return confirmationOutput{Message: resp.Message}, nil
}

// Suppressions.

type suppressionOutput struct {
	ID        string    `json:"id" jsonschema:"Suppression ID."`
	Email     string    `json:"email" jsonschema:"Suppressed address."`
	Domain    string    `json:"domain,omitempty" jsonschema:"Sending domain the suppression applies to."`
	Reason    string    `json:"reason,omitempty" jsonschema:"Why the address is suppressed."`
	Protected bool      `json:"protected" jsonschema:"True when the recipient chose it, by unsubscribing or reporting spam."`
	CreatedAt time.Time `json:"created_at" jsonschema:"When the suppression was made."`
	ExpiresAt time.Time `json:"expires_at" jsonschema:"When the suppression ends."`
}

func suppressionFrom(s *responses.Suppression) suppressionOutput {
	return suppressionOutput{
		ID:        s.ID.String(),
		Email:     s.Email,
		Domain:    s.Domain,
		Reason:    s.Reason,
		Protected: s.Protected,
		CreatedAt: s.CreatedAt,
		ExpiresAt: s.ExpiresAt,
	}
}

type listSuppressionsInput struct {
	pageInput
	Email    string     `json:"email,omitempty" jsonschema:"Exact email address."`
	Domain   string     `json:"domain,omitempty" jsonschema:"Sending domain."`
	FromTime *time.Time `json:"from_time,omitempty" jsonschema:"RFC 3339 time; only suppressions created at or after it."`
	ToTime   *time.Time `json:"to_time,omitempty" jsonschema:"RFC 3339 time; only suppressions created at or before it."`
}

type listSuppressionsOutput struct {
	Suppressions []suppressionOutput `json:"suppressions" jsonschema:"This page's suppressions."`
	pageOutput
}

func listSuppressions(ctx context.Context, env *toolEnv, in listSuppressionsInput) (listSuppressionsOutput, error) {
	page, err := in.params()
	if err != nil {
		return listSuppressionsOutput{}, err
	}
	resp, _, err := env.client.SuppressionsAPI.GetSuppressions(ctx, env.accountID, requests.GetSuppressionsParams{
		Email:            optionalString(strings.ToLower(strings.TrimSpace(in.Email))),
		Domain:           optionalString(strings.ToLower(strings.TrimSpace(in.Domain))),
		FromTime:         in.FromTime,
		ToTime:           in.ToTime,
		PaginationParams: page,
	})
	if err != nil {
		return listSuppressionsOutput{}, err
	}
	out := listSuppressionsOutput{Suppressions: []suppressionOutput{}, pageOutput: pageOutputFrom(resp.Pagination)}
	for i := range resp.Data {
		out.Suppressions = append(out.Suppressions, suppressionFrom(&resp.Data[i]))
	}
	return out, nil
}

type addSuppressionInput struct {
	Email     string    `json:"email" jsonschema:"Address to suppress."`
	ExpiresAt time.Time `json:"expires_at" jsonschema:"RFC 3339 time the suppression ends."`
	Domain    string    `json:"domain,omitempty" jsonschema:"Sending domain to suppress it for; omit for each domain of the account."`
	Reason    string    `json:"reason,omitempty" jsonschema:"Why, in up to 255 characters."`
}

type addSuppressionOutput struct {
	Suppressions []suppressionOutput `json:"suppressions" jsonschema:"The suppressions made, one per domain."`
}

func addSuppression(ctx context.Context, env *toolEnv, in addSuppressionInput) (addSuppressionOutput, error) {
	resp, _, err := env.client.SuppressionsAPI.CreateSuppression(ctx, env.accountID, requests.CreateSuppressionRequest{
		Email:     strings.ToLower(strings.TrimSpace(in.Email)),
		ExpiresAt: in.ExpiresAt,
		Domain:    optionalString(strings.ToLower(strings.TrimSpace(in.Domain))),
		Reason:    optionalString(in.Reason),
	})
	if err != nil {
		return addSuppressionOutput{}, err
	}
	out := addSuppressionOutput{Suppressions: []suppressionOutput{}}
	for i := range resp.Data {
		out.Suppressions = append(out.Suppressions, suppressionFrom(&resp.Data[i]))
	}
	return out, nil
}

type removeSuppressionInput struct {
	Email  string `json:"email" jsonschema:"Suppressed address."`
	Domain string `json:"domain,omitempty" jsonschema:"Sending domain to remove the suppression for; omit to remove it for every domain."`
}

func removeSuppression(ctx context.Context, env *toolEnv, in removeSuppressionInput) (confirmationOutput, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if email == "" {
		return confirmationOutput{}, invalidInput("email is required.")
	}
	resp, _, err := env.client.SuppressionsAPI.DeleteSuppression(ctx, env.accountID, email,
		optionalString(strings.ToLower(strings.TrimSpace(in.Domain))))
	if err != nil {
		return confirmationOutput{}, err
	}
	return confirmationOutput{Message: resp.Message}, nil
}

// SMTP credentials.

type smtpCredentialOutput struct {
	ID        string    `json:"id" jsonschema:"SMTP credential ID."`
	Name      string    `json:"name" jsonschema:"Credential name."`
	Username  string    `json:"username" jsonschema:"SMTP username."`
	Sandbox   bool      `json:"sandbox" jsonschema:"True when its sends are sandbox sends, never delivered."`
	Scope     string    `json:"scope" jsonschema:"global for every sending domain, scoped for the domains listed."`
	Domains   []string  `json:"domains,omitempty" jsonschema:"The domains a scoped credential may send from."`
	CreatedAt time.Time `json:"created_at" jsonschema:"When the credential was created."`
}

func smtpCredentialFrom(c *responses.SMTPCredential) smtpCredentialOutput {
	// The password is never returned.
	return smtpCredentialOutput{
		ID:        c.ID.String(),
		Name:      c.Name,
		Username:  c.Username,
		Sandbox:   c.Sandbox,
		Scope:     c.Scope,
		Domains:   c.Domains,
		CreatedAt: c.CreatedAt,
	}
}

type listSMTPCredentialsOutput struct {
	SMTPCredentials []smtpCredentialOutput `json:"smtp_credentials" jsonschema:"This page's credentials."`
	pageOutput
}

func listSMTPCredentials(ctx context.Context, env *toolEnv, in pageInput) (listSMTPCredentialsOutput, error) {
	page, err := in.params()
	if err != nil {
		return listSMTPCredentialsOutput{}, err
	}
	resp, _, err := env.client.SMTPCredentialsAPI.GetSMTPCredentials(ctx, env.accountID, &page)
	if err != nil {
		return listSMTPCredentialsOutput{}, err
	}
	out := listSMTPCredentialsOutput{SMTPCredentials: []smtpCredentialOutput{}, pageOutput: pageOutputFrom(resp.Pagination)}
	for i := range resp.Data {
		out.SMTPCredentials = append(out.SMTPCredentials, smtpCredentialFrom(&resp.Data[i]))
	}
	return out, nil
}

type smtpCredentialIDInput struct {
	SMTPCredentialID string `json:"smtp_credential_id" jsonschema:"SMTP credential ID."`
}

func getSMTPCredential(ctx context.Context, env *toolEnv, in smtpCredentialIDInput) (smtpCredentialOutput, error) {
	id, err := parseID("smtp_credential_id", in.SMTPCredentialID)
	if err != nil {
		return smtpCredentialOutput{}, err
	}
	c, _, err := env.client.SMTPCredentialsAPI.GetSMTPCredential(ctx, env.accountID, id)
	if err != nil {
		return smtpCredentialOutput{}, err
	}
	return smtpCredentialFrom(c), nil
}

func deleteSMTPCredential(ctx context.Context, env *toolEnv, in smtpCredentialIDInput) (confirmationOutput, error) {
	id, err := parseID("smtp_credential_id", in.SMTPCredentialID)
	if err != nil {
		return confirmationOutput{}, err
	}
	resp, _, err := env.client.SMTPCredentialsAPI.DeleteSMTPCredential(ctx, env.accountID, id)
	if err != nil {
		return confirmationOutput{}, err
	}
	return confirmationOutput{Message: resp.Message}, nil
}

// Sub-accounts.

type subAccountOutput struct {
	ID             string     `json:"id" jsonschema:"Sub-account ID."`
	Name           string     `json:"name" jsonschema:"Name."`
	Website        string     `json:"website" jsonschema:"Website."`
	Status         string     `json:"status" jsonschema:"active, suspended, or parent-suspended (this account is suspended)."`
	MonthlyCredit  int64      `json:"monthly_credit" jsonschema:"Monthly credit cap; 0 means no cap."`
	DomainCount    int64      `json:"domain_count" jsonschema:"Sending domains."`
	MemberCount    int64      `json:"member_count" jsonschema:"Team members."`
	LastActivityAt *time.Time `json:"last_activity_at,omitempty" jsonschema:"When it was last active."`
	CreatedAt      time.Time  `json:"created_at" jsonschema:"When it was created."`
}

func subAccountFrom(s *responses.SubAccount) subAccountOutput {
	return subAccountOutput{
		ID:             s.ID.String(),
		Name:           s.Name,
		Website:        s.Website,
		Status:         s.Status,
		MonthlyCredit:  s.MonthlyCredit,
		DomainCount:    s.DomainCount,
		MemberCount:    s.MemberCount,
		LastActivityAt: s.LastActivityAt,
		CreatedAt:      s.CreatedAt,
	}
}

type listSubAccountsOutput struct {
	SubAccounts []subAccountOutput `json:"sub_accounts" jsonschema:"This page's sub-accounts."`
	pageOutput
}

func listSubAccounts(ctx context.Context, env *toolEnv, in pageInput) (listSubAccountsOutput, error) {
	page, err := in.params()
	if err != nil {
		return listSubAccountsOutput{}, err
	}
	resp, _, err := env.client.SubAccountsAPI.ListSubAccounts(ctx, env.accountID, &page)
	if err != nil {
		return listSubAccountsOutput{}, err
	}
	out := listSubAccountsOutput{SubAccounts: []subAccountOutput{}, pageOutput: pageOutputFrom(resp.Pagination)}
	for i := range resp.Data {
		out.SubAccounts = append(out.SubAccounts, subAccountFrom(&resp.Data[i]))
	}
	return out, nil
}

type subAccountIDInput struct {
	SubAccountID string `json:"sub_account_id" jsonschema:"Sub-account ID."`
}

func getSubAccount(ctx context.Context, env *toolEnv, in subAccountIDInput) (subAccountOutput, error) {
	id, err := parseID("sub_account_id", in.SubAccountID)
	if err != nil {
		return subAccountOutput{}, err
	}
	s, _, err := env.client.SubAccountsAPI.GetSubAccount(ctx, env.accountID, id)
	if err != nil {
		return subAccountOutput{}, err
	}
	return subAccountFrom(s), nil
}

type createSubAccountInput struct {
	Name          string `json:"name" jsonschema:"Name."`
	Website       string `json:"website" jsonschema:"A domain or URL, such as example.com; only its host is kept."`
	MonthlyCredit *int64 `json:"monthly_credit,omitempty" jsonschema:"Monthly credit cap, 0 to 1000000000; 0 means no cap."`
}

func createSubAccount(ctx context.Context, env *toolEnv, in createSubAccountInput) (subAccountOutput, error) {
	s, _, err := env.client.SubAccountsAPI.CreateSubAccount(ctx, env.accountID, requests.CreateSubAccountRequest{
		Name:          in.Name,
		Website:       in.Website,
		MonthlyCredit: in.MonthlyCredit,
	})
	if err != nil {
		return subAccountOutput{}, err
	}
	return subAccountFrom(s), nil
}

type updateSubAccountInput struct {
	SubAccountID  string `json:"sub_account_id" jsonschema:"Sub-account ID."`
	Name          string `json:"name,omitempty" jsonschema:"New name."`
	Website       string `json:"website,omitempty" jsonschema:"New website: a domain or URL, such as example.com; only its host is kept."`
	MonthlyCredit *int64 `json:"monthly_credit,omitempty" jsonschema:"New monthly credit cap, 0 to 1000000000; 0 means no cap."`
}

func updateSubAccount(ctx context.Context, env *toolEnv, in updateSubAccountInput) (subAccountOutput, error) {
	id, err := parseID("sub_account_id", in.SubAccountID)
	if err != nil {
		return subAccountOutput{}, err
	}
	s, _, err := env.client.SubAccountsAPI.UpdateSubAccount(ctx, env.accountID, id, requests.UpdateSubAccountRequest{
		Name:          optionalString(in.Name),
		Website:       optionalString(in.Website),
		MonthlyCredit: in.MonthlyCredit,
	})
	if err != nil {
		return subAccountOutput{}, err
	}
	return subAccountFrom(s), nil
}

type usageShare struct {
	AccountID      string  `json:"account_id,omitempty" jsonschema:"Account ID."`
	Name           string  `json:"name,omitempty" jsonschema:"Account name."`
	ReceptionCount int64   `json:"reception_count" jsonschema:"Emails AhaSend accepted this billing period."`
	AllocatedCost  float64 `json:"allocated_cost" jsonschema:"Share of the period's cost."`
}

func usageShareFrom(u responses.SubAccountUsageBreakdown) usageShare {
	out := usageShare{Name: deref(u.Name), ReceptionCount: u.ReceptionCount, AllocatedCost: u.AllocatedCost}
	if u.AccountID != nil {
		out.AccountID = u.AccountID.String()
	}
	return out
}

type subAccountUsageOutput struct {
	PeriodStart        time.Time    `json:"period_start" jsonschema:"Start of the billing period."`
	PeriodEnd          time.Time    `json:"period_end" jsonschema:"End of the billing period."`
	Currency           string       `json:"currency" jsonschema:"Currency of the costs."`
	AllocationMethod   string       `json:"allocation_method" jsonschema:"How the cost is shared."`
	AllocationNote     string       `json:"allocation_note,omitempty" jsonschema:"Notes on the sharing."`
	Parent             usageShare   `json:"parent" jsonschema:"This account's own usage."`
	SubAccounts        []usageShare `json:"sub_accounts" jsonschema:"Usage of each sub-account that is not deleted, suspended ones included."`
	RemovedSubAccounts usageShare   `json:"removed_sub_accounts" jsonschema:"Usage of sub-accounts removed this period, together."`
	Total              usageShare   `json:"total" jsonschema:"Totals."`
}

func getSubAccountUsage(ctx context.Context, env *toolEnv, _ struct{}) (subAccountUsageOutput, error) {
	u, _, err := env.client.SubAccountsAPI.GetSubAccountsUsage(ctx, env.accountID)
	if err != nil {
		return subAccountUsageOutput{}, err
	}
	out := subAccountUsageOutput{
		PeriodStart:        u.BillingPeriod.Start,
		PeriodEnd:          u.BillingPeriod.End,
		Currency:           u.Currency,
		AllocationMethod:   u.AllocationMethod,
		AllocationNote:     u.AllocationNote,
		Parent:             usageShareFrom(u.Parent),
		SubAccounts:        []usageShare{},
		RemovedSubAccounts: usageShareFrom(u.RemovedSubAccounts),
		Total:              usageShareFrom(u.Total),
	}
	for _, s := range u.SubAccounts {
		out.SubAccounts = append(out.SubAccounts, usageShareFrom(s))
	}
	return out, nil
}

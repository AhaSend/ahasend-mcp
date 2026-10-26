# ahasend-mcp

A Go module with [MCP](https://modelcontextprotocol.io) tools for AhaSend.
With these tools, an AI agent can send email, find out why an email failed,
and manage domains, templates, contacts, lists, campaigns, webhooks, routes,
suppressions, SMTP credentials and sub-accounts.

Each tool calls the AhaSend v2 API through
[ahasend-go](https://github.com/AhaSend/ahasend-go). The API key's scopes
decide what the agent can do.

The module builds a server with the
[official Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk). Your
program chooses how to serve it: stdio, HTTP, or any other SDK transport.

```sh
go get github.com/AhaSend/ahasend-mcp
```

## Example

```go
server, err := ahasendmcp.NewServer(
	&mcp.Implementation{Name: "my-app", Version: "1.0.0"},
	ahasendmcp.Options{
		Client:           api.NewAPIClient(api.WithAPIKey(apiKey)),
		AccountID:        accountID,
		DashboardBaseURL: "https://dash.ahasend.com",
		ConfirmationKey:  key, // at least 32 random bytes
	},
)
if err != nil {
	return err
}
return server.Run(ctx, &mcp.StdioTransport{})
```

## Options

| Field | What it does |
|---|---|
| `Client` | The ahasend-go client, with the API key. Required. |
| `AccountID` | The account the tools work on. Required. |
| `DashboardBaseURL` | The AhaSend dashboard URL, used in campaign links. Required. |
| `ConfirmationKey` | A secret of at least 32 bytes that signs campaign send questions. Required. |
| `Scopes` | The API key's scopes. Only tools the key can use are shown. `nil` shows every tool. |
| `Toolsets` | Show only these toolsets. `nil` shows all. |
| `ReadOnly` | Show only tools that change nothing. |
| `Stateless` | Set to true if you serve over stateless HTTP. |
| `MaxRecipientsPerSend` | The most recipients for one send. Default 50. |
| `ReadInbound` | Let the message tools return email the account received. Off by default. |
| `Blocked` | If set, every tool call returns this text as an error and does nothing. |
| `Recorder` | Gets one record per tool call: tool name, success, error kind and time taken. Never the inputs or results. |

If you serve many users, build a new server for each request with that
user's key and scopes. This is cheap. Use the same `ConfirmationKey` on
every instance.

`Catalogue()` lists every tool with the scopes it needs.
`OfferedScopes()` lists every scope the tools use, for a page where a user
picks scopes.

## Tools

| Toolset | Tools |
|---|---|
| `sending` | `send_email`, `send_conversation_email`, `send_template_email`, `cancel_scheduled_email` |
| `diagnostics` | `list_messages`, `get_message`, `diagnose_message`, `get_deliverability_stats`, `get_bounce_stats`, `get_delivery_time_stats` |
| `domains` | `list_domains`, `get_domain`, `add_domain`, `check_domain_dns`, `update_domain`, `delete_domain` |
| `templates` | `list_templates`, `get_template`, `get_published_template`, `create_template`, `update_template`, `delete_template`, `discard_template_draft`, `publish_template`, `list_template_versions`, `get_template_version`, `restore_template_version`, `list_campaign_templates`, `get_campaign_template`, `create_campaign_template`, `update_campaign_template`, `delete_campaign_template` |
| `audience` | `list_contacts`, `get_contact`, `create_contact`, `upsert_contacts`, `update_contact`, `delete_contact`, `list_contact_lists`, `list_lists`, `get_list`, `create_list`, `update_list`, `delete_list`, `list_list_contacts`, `add_contacts_to_list`, `upsert_list_contact`, `remove_contact_from_list`, `list_segments`, `get_segment`, `create_segment`, `update_segment`, `delete_segment`, `preview_segment`, `list_segment_contacts`, `list_contact_segments` |
| `campaigns` | `list_campaigns`, `get_campaign`, `create_campaign`, `update_campaign`, `delete_campaign`, `duplicate_campaign`, `send_campaign_test`, `send_campaign`, `schedule_campaign`, `unschedule_campaign`, `get_campaign_stats`, `list_campaign_recipients` |
| `management` | `get_account`, `list_webhooks`, `get_webhook`, `create_webhook`, `update_webhook`, `delete_webhook`, `list_routes`, `get_route`, `create_route`, `update_route`, `delete_route`, `list_suppressions`, `add_suppression`, `remove_suppression`, `list_smtp_credentials`, `get_smtp_credential`, `delete_smtp_credential`, `list_sub_accounts`, `get_sub_account`, `create_sub_account`, `update_sub_account`, `get_sub_account_usage` |

Each tool needs the scopes its API endpoint needs. `send_campaign` and
`schedule_campaign` also need `campaigns:read` and `lists:read`, and
`segments:read` to send a campaign that uses segments. The campaign
template tools also need `campaigns:read`. `preview_segment`,
`list_segment_contacts` and `list_contact_segments` need `contacts:read`
as well as `segments:read`.

List tools return 20 items by default and at most 100. Pass the returned
cursor to get the next page.

There are no tools to create API keys or SMTP credentials, change account
settings, billing or team members, remove all suppressions, or delete or
suspend sub-accounts.

## Simple Editor designs and AhaML

The template, campaign template and campaign tools read and write a design
of the dashboard's Simple Editor as AhaSend Markup (AhaML), an XML form of
the design, in the content's `markup` field. `get_template` returns a
template's working copy, the draft while one exists, so an agent edits what
it read; `get_published_template` returns the copy a send uses.

The server has one resource, `ahasend://docs/ahaml`: the AhaML reference,
with every tag, attribute, limit and error code. It is also at
<https://ahasend.com/docs/send-api/ahaml>. The resource is listed on every
connection, so an agent without web access can read it.

## Safety rules

**Campaign sends need a person's yes.** `send_campaign` and
`schedule_campaign` show the user a form with the send time, the number of
contacts, the campaign name, subject, sender, lists and segments, and the
excluded lists and segments. They send only if the user answers yes. If
the campaign changes after the question, the tool asks again. If the client
cannot show a form, the tool sends nothing and returns a link to the
campaign in the dashboard.

**One-off sends have a recipient limit.** A single send to more than
`MaxRecipientsPerSend` recipients (default 50) is refused. Use a campaign
for bulk email.

**Received email is hidden by default.** Anyone can send email to an
account's inbound addresses, and that email can contain text that tries to
trick the agent. Unless `ReadInbound` is on, the message tools do not return
received email. When it is on, received messages are marked
`untrusted: true`.

## Errors

When the API refuses a call, the tool returns an error with the API's
message and what to do next. When it refuses AhaML, the error lists the
problems, up to the first 100, each with its code, line, column, path and
hint; the API's message gives how many there are.

## Trying it locally

`cmd/ahasend-mcp-dev` serves every tool over stdio with your own API key.
It is for testing only.

```sh
go build -o ahasend-mcp-dev ./cmd/ahasend-mcp-dev
claude mcp add ahasend \
  -e AHASEND_API_KEY=... -e AHASEND_ACCOUNT_ID=... \
  -- ./ahasend-mcp-dev
```

Set `AHASEND_BASE_URL` to use an API other than the default.

## License

MIT

# ahasend-mcp

Go module with the AhaSend [MCP](https://modelcontextprotocol.io) tools. It
lets AI agents send email, check delivery, and manage domains, templates,
contacts, lists, campaigns, webhooks, routes, suppressions, SMTP credentials
and sub-accounts through the AhaSend v2 API.

Every tool calls the v2 API through
[ahasend-go](https://github.com/AhaSend/ahasend-go), so the API key's scopes
and the API's own rules decide what a connection may do. The module adds a
few rules of its own for agents: which tools a connection lists, a limit on
one-off sends, a human confirmation before any campaign send, and a switch
for received email.

The module builds an `*mcp.Server` of the
[official Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk). It
does not open a transport itself: the program that uses it serves the server
over stdio, Streamable HTTP or any other SDK transport.

```sh
go get github.com/AhaSend/ahasend-mcp
```

## Using the module

`NewServer` returns a server with the tools that `Options` allow:

```go
server, err := ahasendmcp.NewServer(
	&mcp.Implementation{Name: "my-app", Version: "1.0.0"},
	ahasendmcp.Options{
		Client:           api.NewAPIClient(api.WithAPIKey(apiKey)),
		AccountID:        accountID,
		Scopes:           nil, // the key's scopes are not known: list every tool
		DashboardBaseURL: "https://dash.ahasend.com",
		ConfirmationKey:  key, // 32 random bytes or more
	},
)
if err != nil {
	return err
}
return server.Run(ctx, &mcp.StdioTransport{})
```

`NewServer` returns an error for a nil `impl`, a nil `Client`, a zero
`AccountID`, an empty `DashboardBaseURL`, or a `ConfirmationKey` shorter
than 32 bytes.

### A command-line tool

A local program, such as a CLI that serves the tools over stdio with the
user's own API key, builds one server per process. When it cannot read the
key's scopes it leaves `Scopes` nil: every tool is listed, and the API
refuses what the key may not do. It passes a random `ConfirmationKey` per
process, leaves `Stateless` false, and may narrow the tools with `Toolsets`
and `ReadOnly`.

### A hosted server

A server that serves many users over Streamable HTTP builds a new server
with `NewServer` for every connection or request, from that connection's
credentials: its own `api.APIClient`, account and the key's current
`Scopes` (an empty, non-nil slice when the key has none). Servers built this
way share one schema cache, so building one per request is cheap. A
stateless HTTP handler sets `Stateless` to true, and every instance passes
the same `ConfirmationKey`, so a campaign confirmation started on one
instance completes on another.

`Catalogue()` returns every tool's name, title, toolset, required scopes and
annotations, and `OfferedScopes()` returns the API scopes a connection needs
for the whole catalogue (see "Required scopes"), for a consent or key
creation page.

## Options

| Field | Meaning |
|---|---|
| `Client` | ahasend-go client, with the base URL and the API key. Required. |
| `AccountID` | The account the tools act on. Required. |
| `Scopes` | The key's API scopes, which decide the tools listed. `nil` means unknown: every tool is listed. An empty slice lists none. |
| `Toolsets` | Toolset names to list; `nil` lists every toolset. Unknown names are ignored. |
| `ReadOnly` | True lists only read-only tools. |
| `DashboardBaseURL` | Base URL of the AhaSend dashboard, for the campaign confirmation link. Required. |
| `ConfirmationKey` | HMAC key that signs campaign confirmations, at least 32 bytes. Required. |
| `Stateless` | True when the server is served statelessly, so it cannot send requests to clients on protocol versions before 2026-07-28. |
| `MaxRecipientsPerSend` | The one-off send limit; 0 means 50. |
| `ReadInbound` | True lets the message tools return received email. Off by default. |
| `Blocked` | When set, every listed tool's call returns this text as a tool error and makes no API call. |
| `Recorder` | Receives one `ToolCall` per tool call: the tool, its toolset, success, an error kind and the duration, never the arguments or the result. |

## Tool visibility

A tool is listed only when all three hold:

1. The key's scopes meet every scope the tool requires, or `Scopes` is nil.
2. The tool's toolset is in `Toolsets`, when that is set.
3. The tool is read-only, when `ReadOnly` is set.

A call of a tool that is not listed gets the MCP "unknown tool" error. The
tool list is cacheable for 5 minutes, privately to the connection.

## Required scopes

A tool's required scopes are the API scopes its endpoint checks, plus:

- `campaigns:read` and `lists:read` on `send_campaign` and
  `schedule_campaign`, whose confirmation shows the campaign and its lists;
- `campaigns:read` on the campaign template tools, whose endpoints exist
  only for accounts with marketing on.

Scopes that take a sending domain are written as a family without the
domain: `messages:read` is met by `messages:read:all` or by any
`messages:read:{domain}`. The families are `messages:send`,
`messages:cancel`, `messages:read`, `domains:delete`, `webhooks:read`,
`webhooks:write`, `webhooks:delete`, `routes:read`, `routes:write`,
`routes:delete`, `smtp-credentials:read`, `smtp-credentials:write`,
`smtp-credentials:delete` and `statistics-transactional:read`. Every other
scope is met only by itself.

## Toolsets and tools

Kinds: **read** is read-only; **write** changes something and destroys
nothing; **destructive** removes or ends something; **open world** reaches
people outside the account; **idempotent** can be repeated without further
effect. Every tool sets all four MCP annotations.

List tools take `limit` (default 20, at most 100) and `cursor`, and return
the API's `next_cursor`. Long text, such as message bodies, template content
and campaign content, is returned only by the `get_*` tools.

### sending

| Tool | Required scopes | Kind |
|---|---|---|
| `send_email` | `messages:send` | write, open world |
| `send_conversation_email` | `messages:send` | write, open world |
| `send_template_email` | `messages:send` | write, open world |
| `cancel_scheduled_email` | `messages:cancel` | destructive |

### diagnostics

| Tool | Required scopes | Kind |
|---|---|---|
| `list_messages` | `messages:read` | read |
| `get_message` | `messages:read` | read |
| `diagnose_message` | `messages:read` | read |
| `get_deliverability_stats` | `statistics-transactional:read` | read |
| `get_bounce_stats` | `statistics-transactional:read` | read |
| `get_delivery_time_stats` | `statistics-transactional:read` | read |

`diagnose_message` also reads the recipient's suppressions and the sender
domain when the key allows it, and lists each check it could not make.

### domains

| Tool | Required scopes | Kind |
|---|---|---|
| `list_domains` | `domains:read` | read |
| `get_domain` | `domains:read` | read |
| `add_domain` | `domains:write` | write |
| `check_domain_dns` | `domains:write` | write, idempotent |
| `update_domain` | `domains:write` | write |
| `delete_domain` | `domains:delete` | destructive |

### templates

| Tool | Required scopes | Kind |
|---|---|---|
| `list_templates` | `templates:read` | read |
| `get_template` | `templates:read` | read |
| `create_template` | `templates:write` | write |
| `update_template` | `templates:write` | write |
| `delete_template` | `templates:delete` | destructive |
| `get_template_draft` | `templates:read` | read |
| `discard_template_draft` | `templates:write` | destructive |
| `publish_template` | `templates:write` | write |
| `list_template_versions` | `templates:read` | read |
| `get_template_version` | `templates:read` | read |
| `restore_template_version` | `templates:write` | write |
| `list_campaign_templates` | `templates:read`, `campaigns:read` | read |
| `get_campaign_template` | `templates:read`, `campaigns:read` | read |
| `create_campaign_template` | `templates:write`, `campaigns:read` | write |
| `update_campaign_template` | `templates:write`, `campaigns:read` | write |
| `delete_campaign_template` | `templates:delete`, `campaigns:read` | destructive |

Template content is HTML or MJML with an optional plain text version. For a
design made in the dashboard's visual editor (editor `simple`), the API
refuses HTML and MJML; its name, subject, preview text, sender and plain
text can still change. `list_template_versions` returns
every kept version (at most 50) and takes no `limit` or `cursor`.

### audience

| Tool | Required scopes | Kind |
|---|---|---|
| `list_contacts` | `contacts:read` | read |
| `get_contact` | `contacts:read` | read |
| `create_contact` | `contacts:write` | write |
| `upsert_contacts` | `contacts:write` | write |
| `update_contact` | `contacts:write` | write |
| `delete_contact` | `contacts:delete` | destructive |
| `list_contact_lists` | `lists:read` | read |
| `list_lists` | `lists:read` | read |
| `get_list` | `lists:read` | read |
| `create_list` | `lists:write` | write |
| `update_list` | `lists:write` | write |
| `delete_list` | `lists:delete` | destructive |
| `list_list_contacts` | `lists:read` | read |
| `add_contacts_to_list` | `lists:write` | write |
| `upsert_list_contact` | `lists:write` | write |
| `remove_contact_from_list` | `lists:write` | destructive |

### campaigns

| Tool | Required scopes | Kind |
|---|---|---|
| `list_campaigns` | `campaigns:read` | read |
| `get_campaign` | `campaigns:read` | read |
| `create_campaign` | `campaigns:write` | write |
| `update_campaign` | `campaigns:write` | write |
| `delete_campaign` | `campaigns:delete` | destructive |
| `duplicate_campaign` | `campaigns:write` | write |
| `send_campaign_test` | `campaigns:send` | write, open world |
| `send_campaign` | `campaigns:send`, `campaigns:read`, `lists:read` | write, open world |
| `schedule_campaign` | `campaigns:send`, `campaigns:read`, `lists:read` | write, open world |
| `unschedule_campaign` | `campaigns:send` | write |
| `get_campaign_stats` | `campaigns:read` | read |
| `list_campaign_recipients` | `campaigns:read` | read |

`get_campaign` returns the API's `ready` flag and `issues`, so an agent can
fix a campaign before it asks to send it. `send_campaign` and
`schedule_campaign` always ask the user first (see "Campaign send
confirmation"); `send_campaign_test` and `unschedule_campaign` do not.

### management

| Tool | Required scopes | Kind |
|---|---|---|
| `get_account` | `accounts:read` | read |
| `list_webhooks` | `webhooks:read` | read |
| `get_webhook` | `webhooks:read` | read |
| `create_webhook` | `webhooks:write` | write |
| `update_webhook` | `webhooks:write` | write |
| `delete_webhook` | `webhooks:delete` | destructive |
| `list_routes` | `routes:read` | read |
| `get_route` | `routes:read` | read |
| `create_route` | `routes:write` | write |
| `update_route` | `routes:write` | write |
| `delete_route` | `routes:delete` | destructive |
| `list_suppressions` | `suppressions:read` | read |
| `add_suppression` | `suppressions:write` | write |
| `remove_suppression` | `suppressions:delete` | destructive |
| `list_smtp_credentials` | `smtp-credentials:read` | read |
| `get_smtp_credential` | `smtp-credentials:read` | read |
| `delete_smtp_credential` | `smtp-credentials:delete` | destructive |
| `list_sub_accounts` | `sub-accounts:read` | read |
| `get_sub_account` | `sub-accounts:read` | read |
| `create_sub_account` | `sub-accounts:write` | write |
| `update_sub_account` | `sub-accounts:write` | write |
| `get_sub_account_usage` | `sub-accounts:usage` | read |

`create_webhook` and `create_route` return the signing secret, which the API
returns only on creation. No tool returns an SMTP password. There are no
tools that create API keys or SMTP credentials, change account settings,
billing or members, wipe all suppressions, or delete or suspend
sub-accounts.

## Campaign send confirmation

`send_campaign` and `schedule_campaign` never send without a human's yes,
whatever the key's scopes.

A client **can be asked** when it declares form elicitation (an
`elicitation` capability that names `form`, or names neither `form` nor
`url`) and the server can reach it: always on protocol version 2026-07-28 or
later, and on an older version only when `Options.Stateless` is false.

Before it asks, the tool refuses a campaign that is not a draft, a campaign
the API marks not ready (with its issues), a campaign whose lists hold no
reachable contacts, and a `send_at` less than a minute ahead. These checks
avoid the common refusals; the API can still refuse the send after the
user's yes, for example for a paused sender domain or lists whose contacts
are not reachable when it checks them.

- **A client that can be asked.** The first call reads the campaign and its
  lists and returns an input request (a multi round-trip request) with a
  form that shows the send time, the total contact count, each list's name
  and contact count, and the campaign's name, subject and sender, with one
  yes/no field. Every value an agent can write is shown on one line, in
  double quotes, without control or bidi characters, and cut to 100
  characters. The result also carries a `requestState` signed with
  HMAC-SHA256 under `ConfirmationKey`, which binds the answer to the tool,
  account, campaign and send time and to the campaign as it was asked
  about, and expires after 10 minutes. On the
  call with the answer, the tool reads the campaign again and sends only
  when the state is valid and unexpired, matches the call, the campaign has
  not changed since the question, and the user answered yes. A changed
  campaign, or an answer without a valid state, is asked about again. A no,
  a declined form and a cancelled form send nothing. On a protocol version
  before 2026-07-28 the SDK asks the client through the session and the
  tool completes in the same call, so it cannot ask a second question: when
  the answer does not match the question (the campaign changed, or the
  answer was late or not valid), the tool sends nothing and its result says
  to call it again.
- **A client that cannot be asked.** The tool sends nothing and returns the
  campaign's dashboard URL, `{DashboardBaseURL}/account/{account_id}/marketing/campaigns/{campaign_id}`,
  where the user sends it.

When `Scopes` is nil and the key may not read the campaign or one of its
lists, the tool also sends nothing and returns the dashboard URL.

The `Recorder` records a question round and a dashboard URL result as a
success, and a no, a declined form or a cancelled form with the error kind
`declined`.

## Received email

The API's message endpoints return received email (any message whose
`direction` is not `outbound`) beside outbound mail. Anyone can send mail to
an account's inbound addresses, so received email can carry text that tries
to steer the agent.

With `ReadInbound` off (the default), `list_messages` drops received
messages from each page and counts them in `received_hidden`, so a page can
hold fewer messages than `limit`, or none, and still carry a next cursor;
`get_message` refuses a received message without returning any of it. With
`ReadInbound` on, both return received messages marked `untrusted: true`.
`diagnose_message` works only on outbound messages either way.

## One-off send limit

`send_email` and `send_template_email` refuse more than
`MaxRecipientsPerSend` recipients (default 50), and
`send_conversation_email` more than that many `to`, `cc` and `bcc`
addresses together, before any API call. Bulk email belongs in a campaign.

## Errors

API errors come back as tool results with `isError` set and a message that
says what to do next. The `Recorder` gets one of these error kinds:
`api_4xx`, `api_5xx` (also a network failure), `invalid_input` (the module
refused the call before making the API call it was asked for, such as a send
over the one-off send limit or a campaign send for a campaign that is not a
draft), `declined`, `inbound_refused` and `blocked`.

## License

MIT

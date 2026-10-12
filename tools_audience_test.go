package ahasendmcp

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAudienceTools(t *testing.T) {
	list := "/lists/" + testListID

	runToolCases(t, []toolCase{
		{
			tool:   "list_contacts",
			args:   map[string]any{"email": " Ann@Example.ORG ", "status": "enabled", "subscribed": true, "from_time": fixtureTime},
			method: http.MethodGet, path: "/contacts",
			wantQuery: map[string]string{
				"email": "ann@example.org", "status": "enabled", "subscribed": "true", "from_time": fixtureTime, "limit": "20",
			},
			resp: page(contactJSON()),
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, testContactID, field(t, out, "contacts.0.id"))
				assert.Equal(t, "pro", field(t, out, "contacts.0.attributes.plan"))
				assert.Equal(t, "next-page", out["next_cursor"])
			},
		},
		{
			tool: "get_contact", args: map[string]any{"contact": "Ann@Example.org"},
			method: http.MethodGet, path: "/contacts/ann@example.org", resp: contactJSON(),
			check: func(t *testing.T, out map[string]any) { assert.Equal(t, "valid", out["validation_status"]) },
		},
		{
			tool: "create_contact",
			args: map[string]any{
				"email": "Ann@Example.org", "first_name": "Ann", "unsubscribed": false,
				"attributes": map[string]any{"plan": "pro"},
			},
			method: http.MethodPost, path: "/contacts", status: http.StatusCreated, resp: contactJSON(),
			wantBody: map[string]any{
				"email": "ann@example.org", "first_name": "Ann", "unsubscribed": false,
				"attributes": map[string]any{"plan": "pro"},
			},
			check: func(t *testing.T, out map[string]any) { assert.Equal(t, testContactID, out["id"]) },
		},
		{
			tool: "upsert_contacts",
			args: map[string]any{"contacts": []map[string]any{
				{"email": "ann@example.org", "last_name": "Lee", "attributes": map[string]any{"old": nil}},
				{"email": "not-an-address"},
			}},
			method: http.MethodPost, path: "/contacts/batch",
			resp: map[string]any{
				"object": "list", "created": 0, "updated": 1, "failed": 1,
				"data": []map[string]any{
					{"position": 0, "email": "ann@example.org", "outcome": "updated", "contact": contactJSON()},
					{"position": 1, "email": "not-an-address", "outcome": "failed", "reason": "invalid email"},
				},
			},
			wantBody: map[string]any{"data": []any{
				map[string]any{"email": "ann@example.org", "last_name": "Lee", "attributes": map[string]any{"old": nil}},
				map[string]any{"email": "not-an-address"},
			}},
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, float64(1), out["updated"])
				assert.Equal(t, testContactID, field(t, out, "results.0.contact_id"))
				assert.Equal(t, "invalid email", field(t, out, "results.1.reason"))
			},
		},
		{
			tool:   "update_contact",
			args:   map[string]any{"contact": testContactID, "email": "Ann.Lee@Example.org", "unsubscribed": true},
			method: http.MethodPut, path: "/contacts/" + testContactID, resp: contactJSON(),
			wantBody: map[string]any{"email": "ann.lee@example.org", "unsubscribed": true},
		},
		{
			tool: "delete_contact", args: map[string]any{"contact": "ann@example.org"},
			method: http.MethodDelete, path: "/contacts/ann@example.org", resp: success("contact deleted"),
			check: func(t *testing.T, out map[string]any) { assert.Equal(t, "contact deleted", out["message"]) },
		},
		{
			tool:   "list_contact_lists",
			args:   map[string]any{"contact": "ann@example.org", "subscription_status": "confirmed"},
			method: http.MethodGet, path: "/contacts/ann@example.org/lists",
			wantQuery: map[string]string{"subscription_status": "confirmed"},
			resp:      page(membershipJSON()),
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, "Newsletter", field(t, out, "memberships.0.list_name"))
				assert.Equal(t, "confirmed", field(t, out, "memberships.0.subscription_status"))
			},
		},
		{
			tool: "list_lists", args: map[string]any{"name": "news"},
			method: http.MethodGet, path: "/lists", wantQuery: map[string]string{"name": "news"},
			resp: page(listJSON(testListID, "Newsletter", 120)),
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, float64(120), field(t, out, "lists.0.contact_count"))
			},
		},
		{
			tool: "get_list", args: map[string]any{"list_id": testListID},
			method: http.MethodGet, path: list, resp: listJSON(testListID, "Newsletter", 120),
			check: func(t *testing.T, out map[string]any) { assert.Equal(t, "Newsletter", out["name"]) },
		},
		{
			tool: "create_list", args: map[string]any{"name": "Newsletter", "tags": []string{"news"}},
			method: http.MethodPost, path: "/lists", status: http.StatusCreated, resp: listJSON(testListID, "Newsletter", 0),
			wantBody: map[string]any{"name": "Newsletter", "tags": []any{"news"}},
			check:    func(t *testing.T, out map[string]any) { assert.Equal(t, testListID, out["id"]) },
		},
		{
			tool: "update_list", args: map[string]any{"list_id": testListID, "description": "Weekly", "tags": []string{}},
			method: http.MethodPut, path: list, resp: listJSON(testListID, "Newsletter", 120),
			wantBody: map[string]any{"description": "Weekly", "tags": []any{}},
		},
		{
			tool: "delete_list", args: map[string]any{"list_id": testListID},
			method: http.MethodDelete, path: list, resp: success("list deleted"),
		},
		{
			tool:   "list_list_contacts",
			args:   map[string]any{"list_id": testListID, "email": "ANN@example.org", "subscription_status": "unsubscribed"},
			method: http.MethodGet, path: list + "/contacts",
			wantQuery: map[string]string{"email": "ann@example.org", "subscription_status": "unsubscribed"},
			noQuery:   []string{"include_contacts"},
			resp:      page(membershipJSON()),
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, testContactID, field(t, out, "memberships.0.contact_id"))
			},
		},
		{
			tool:   "add_contacts_to_list",
			args:   map[string]any{"list_id": testListID, "contacts": []string{testContactID, "Bob@Example.org"}},
			method: http.MethodPost, path: list + "/contacts/batch",
			resp: map[string]any{
				"object": "list", "added": 1, "skipped": 0, "failed": 1,
				"data": []map[string]any{
					{"position": 0, "id": testContactID, "outcome": "added", "membership": membershipJSON()},
					{"position": 1, "email": "bob@example.org", "outcome": "not_found", "reason": "contact not found"},
				},
			},
			wantBody: map[string]any{"data": []any{
				map[string]any{"id": testContactID},
				map[string]any{"email": "bob@example.org"},
			}},
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, float64(1), out["added"])
				assert.Equal(t, testContactID, field(t, out, "results.0.contact"))
				assert.Equal(t, "not_found", field(t, out, "results.1.outcome"))
			},
		},
		{
			tool:   "upsert_list_contact",
			args:   map[string]any{"list_id": testListID, "contact": "ann@example.org", "subscription_status": "unsubscribed"},
			method: http.MethodPut, path: list + "/contacts/ann@example.org", resp: membershipJSON(),
			wantBody: map[string]any{"subscription_status": "unsubscribed"},
			check:    func(t *testing.T, out map[string]any) { assert.Equal(t, testListID, out["list_id"]) },
		},
		{
			tool:   "remove_contact_from_list",
			args:   map[string]any{"list_id": testListID, "contact": testContactID},
			method: http.MethodDelete, path: list + "/contacts/" + testContactID, resp: success("contact removed"),
		},
	})
}

func TestContactToolsRefuseAnEmptyContact(t *testing.T) {
	f := newFakeAPI(t)
	cs := connect(t, testOptions(t, f))
	text := requireToolError(t, callTool(t, cs, "get_contact", map[string]any{"contact": " "}))
	assert.Contains(t, text, "contact")
	assert.Empty(t, f.calls())
}

func TestAddContactsToListRefusesAMalformedID(t *testing.T) {
	f := newFakeAPI(t)
	rec := &fakeRecorder{}
	opts := testOptions(t, f)
	opts.Recorder = rec
	cs := connect(t, opts)

	malformed := "6d4e5f6a-7b8c-4d9e-8f0a-2b3c4d5e6fzz"
	text := requireToolError(t, callTool(t, cs, "add_contacts_to_list", map[string]any{
		"list_id": testListID, "contacts": []string{"ann@example.org", malformed},
	}))
	assert.Contains(t, text, "contacts[1]")
	assert.Contains(t, text, "malformed contact ID")
	assert.Empty(t, f.calls())
	assert.Equal(t, ErrorKindInvalidInput, rec.only(t).ErrorKind)
}

func TestAddContactsToListReturnsEachMembershipStatus(t *testing.T) {
	f := newFakeAPI(t)
	unsubscribed := membershipJSON()
	unsubscribed["subscription_status"] = "unsubscribed"
	f.handle("POST "+apiPath("/lists/"+testListID+"/contacts/batch"), http.StatusOK, map[string]any{
		"object": "list", "added": 1, "skipped": 1, "failed": 0,
		"data": []map[string]any{
			{"position": 0, "email": "bob@example.org", "outcome": "added", "membership": membershipJSON()},
			{"position": 1, "email": "ann@example.org", "outcome": "already_member", "membership": unsubscribed},
		},
	})
	cs := connect(t, testOptions(t, f))

	out := decode[map[string]any](t, callTool(t, cs, "add_contacts_to_list", map[string]any{
		"list_id": testListID, "contacts": []string{"bob@example.org", "ann@example.org"},
	}))

	assert.Equal(t, "confirmed", field(t, out, "results.0.subscription_status"))
	assert.Equal(t, "already_member", field(t, out, "results.1.outcome"))
	assert.Equal(t, "unsubscribed", field(t, out, "results.1.subscription_status"))
}

package ahasendmcp

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSegmentID  = "4f2a3b4c-5d6e-4f7a-9b8c-b4c5d6e7f809"
	testSegment2ID = "5a3b4c5d-6e7f-4a8b-8c9d-c5d6e7f8091a"
)

// segmentDefinitionJSON is a definition with a condition, a group and a
// campaign_count value, as the API returns it.
func segmentDefinitionJSON() map[string]any {
	return map[string]any{
		"schema_version": 1, "match": "all",
		"conditions": []any{
			map[string]any{"field": "list", "op": "in", "value": testListID},
			map[string]any{"match": "any", "conditions": []any{
				map[string]any{"field": "attributes.country", "op": "equal", "value": "DE"},
				map[string]any{"field": "campaign_count", "op": "opened_at_least", "value": map[string]any{"count": 3, "since": "90 days ago"}},
				map[string]any{"field": "email", "op": "is_not_empty"},
			}},
		},
	}
}

// segmentJSON is a segment in state; marketable is its marketable_count,
// which only get_segment returns.
func segmentJSON(id, name, state string, marketable any) map[string]any {
	s := map[string]any{
		"object": "segment", "id": id, "created_at": fixtureTime, "updated_at": fixtureTime,
		"name": name, "description": "Engaged customers", "definition": segmentDefinitionJSON(),
		"timezone": "Europe/Berlin", "state": state, "failure_reason": nil, "member_count": 420,
		"last_evaluated_at": fixtureTime,
	}
	if marketable != nil {
		s["marketable_count"] = marketable
	}
	return s
}

// definitionArgs is segmentDefinitionJSON as an agent writes it, without
// schema_version.
func definitionArgs() map[string]any {
	d := segmentDefinitionJSON()
	delete(d, "schema_version")
	return d
}

func TestSegmentTools(t *testing.T) {
	segment := "/segments/" + testSegmentID

	runToolCases(t, []toolCase{
		{
			tool: "list_segments", args: map[string]any{"limit": 5, "cursor": "abc"},
			method: http.MethodGet, path: "/segments", wantQuery: map[string]string{"limit": "5", "after": "abc"},
			resp: page(segmentJSON(testSegmentID, "Engaged", "ready", nil)),
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, testSegmentID, field(t, out, "segments.0.id"))
				assert.Equal(t, float64(420), field(t, out, "segments.0.member_count"))
				assert.Nil(t, field(t, out, "segments.0.definition"), "list results hold no rules")
				assert.Equal(t, "next-page", out["next_cursor"])
			},
		},
		{
			tool: "get_segment", args: map[string]any{"segment_id": testSegmentID},
			method: http.MethodGet, path: segment, resp: segmentJSON(testSegmentID, "Engaged", "ready", 380),
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, float64(380), out["marketable_count"])
				assert.Equal(t, "ready", out["state"])
				assert.Equal(t, testListID, field(t, out, "definition.conditions.0.value"))
				assert.Equal(t, "any", field(t, out, "definition.conditions.1.match"))
				assert.Equal(t, float64(3), field(t, out, "definition.conditions.1.conditions.1.value.count"))
				assert.Nil(t, field(t, out, "definition.conditions.1.conditions.2.value"))
			},
		},
		{
			tool:   "create_segment",
			args:   map[string]any{"name": "Engaged", "definition": definitionArgs(), "timezone": " Europe/Berlin "},
			method: http.MethodPost, path: "/segments", status: http.StatusCreated,
			resp: segmentJSON(testSegmentID, "Engaged", "building", nil),
			wantBody: map[string]any{
				"name": "Engaged", "timezone": "Europe/Berlin", "definition": jsonValue(t, segmentDefinitionJSON()),
			},
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, "building", out["state"])
				assert.NotContains(t, out, "marketable_count")
			},
		},
		{
			tool:   "update_segment",
			args:   map[string]any{"segment_id": testSegmentID, "description": ""},
			method: http.MethodPut, path: segment, resp: segmentJSON(testSegmentID, "Engaged", "ready", nil),
			wantBody: map[string]any{"description": ""},
		},
		{
			tool: "delete_segment", args: map[string]any{"segment_id": testSegmentID},
			method: http.MethodDelete, path: segment, resp: success("segment deleted"),
			check: func(t *testing.T, out map[string]any) { assert.Equal(t, "segment deleted", out["message"]) },
		},
		{
			tool:   "preview_segment",
			args:   map[string]any{"definition": map[string]any{"match": "any", "conditions": []any{map[string]any{"field": "attributes.vip", "op": "is_true"}}}},
			method: http.MethodPost, path: "/segments/preview",
			resp: map[string]any{"object": "segment_preview", "count": nil, "contacts": []any{
				map[string]any{"contact_id": testContactID, "email": "ann@example.org"},
			}},
			wantBody: map[string]any{"definition": map[string]any{
				"schema_version": float64(1), "match": "any",
				"conditions": []any{map[string]any{"field": "attributes.vip", "op": "is_true"}},
			}},
			check: func(t *testing.T, out map[string]any) {
				assert.NotContains(t, out, "count", "a count that timed out is missing, not 0")
				assert.Equal(t, "ann@example.org", field(t, out, "contacts.0.email"))
			},
		},
		{
			tool: "list_segment_contacts", args: map[string]any{"segment_id": testSegmentID},
			method: http.MethodGet, path: segment + "/contacts", wantQuery: map[string]string{"limit": "20"},
			resp: page(map[string]any{
				"object": "segment_contact", "segment_id": testSegmentID, "contact_id": testContactID,
				"email": "ann@example.org", "entered_at": fixtureTime,
			}),
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, testContactID, field(t, out, "contacts.0.contact_id"))
				assert.Equal(t, fixtureTime, field(t, out, "contacts.0.entered_at"))
			},
		},
		{
			tool: "list_contact_segments", args: map[string]any{"contact": " Ann@Example.org "},
			method: http.MethodGet, path: "/contacts/ann@example.org/segments",
			resp: page(segmentJSON(testSegmentID, "Engaged", "ready", nil)),
			check: func(t *testing.T, out map[string]any) {
				assert.Equal(t, "Engaged", field(t, out, "segments.0.name"))
			},
		},
	})
}

func TestUpdateSegmentSendsANewDefinition(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("PUT "+apiPath("/segments/"+testSegmentID), http.StatusOK, segmentJSON(testSegmentID, "Engaged", "building", nil))
	cs := connect(t, testOptions(t, f))

	decode[map[string]any](t, callTool(t, cs, "update_segment", map[string]any{
		"segment_id": testSegmentID, "name": "VIP", "definition": definitionArgs(),
	}))

	assert.Equal(t, map[string]any{"name": "VIP", "definition": jsonValue(t, segmentDefinitionJSON())}, f.onlyCall(t).Body)
}

func TestSegmentDefinitionRefusesAMalformedEntry(t *testing.T) {
	tests := []struct {
		name  string
		entry map[string]any
		want  string
	}{
		{"condition and group", map[string]any{"field": "email", "op": "is_empty", "match": "all", "conditions": []any{}}, "conditions[0] must be a condition"},
		{"neither", map[string]any{}, "conditions[0] must be a condition"},
		{"array value", map[string]any{"field": "email", "op": "equal", "value": []any{"a"}}, "conditions[0].value"},
		{"array value in a group", map[string]any{"match": "all", "conditions": []any{
			map[string]any{"field": "email", "op": "equal", "value": "a"},
			map[string]any{"field": "email", "op": "equal", "value": []any{"a"}},
		}}, "conditions[0].conditions[1].value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeAPI(t)
			rec := &fakeRecorder{}
			opts := testOptions(t, f)
			opts.Recorder = rec
			cs := connect(t, opts)

			text := requireToolError(t, callTool(t, cs, "preview_segment", map[string]any{
				"definition": map[string]any{"match": "all", "conditions": []any{tt.entry}},
			}))

			assert.Contains(t, text, tt.want)
			assert.Empty(t, f.calls())
			assert.Equal(t, ErrorKindInvalidInput, rec.only(t).ErrorKind)
		})
	}
}

func TestSegmentToolsKeepTheAPIRefusal(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("PUT "+apiPath("/segments/"+testSegmentID), http.StatusConflict,
		map[string]any{"message": "The segment is being updated. Try again in a minute."})
	f.handle("POST "+apiPath("/segments"), http.StatusBadRequest,
		map[string]any{"message": "conditions[1].conditions[0].value: unknown contact field country"})
	cs := connect(t, testOptions(t, f))

	text := requireToolError(t, callTool(t, cs, "update_segment", map[string]any{"segment_id": testSegmentID}))
	assert.Contains(t, text, "HTTP 409")
	assert.Contains(t, text, "Try again in a minute")

	text = requireToolError(t, callTool(t, cs, "create_segment", map[string]any{"name": "Engaged", "definition": definitionArgs()}))
	assert.Contains(t, text, "conditions[1].conditions[0].value")
}

func TestSegmentToolsNeedTheirScopes(t *testing.T) {
	opts := testOptions(t, newFakeAPI(t))
	opts.Scopes = []string{"segments:read"}
	names := listedNames(t, connect(t, opts))
	assert.Contains(t, names, "list_segments")
	assert.Contains(t, names, "get_segment")
	for _, name := range []string{"preview_segment", "list_segment_contacts", "list_contact_segments", "create_segment", "delete_segment"} {
		assert.NotContains(t, names, name)
	}

	opts.Scopes = []string{"segments:read", "contacts:read"}
	names = listedNames(t, connect(t, opts))
	for _, name := range []string{"preview_segment", "list_segment_contacts", "list_contact_segments"} {
		assert.Contains(t, names, name)
	}
}

func TestSegmentNotFoundNamesTheMarketingCause(t *testing.T) {
	f := newFakeAPI(t)
	f.handle("GET "+apiPath("/segments"), http.StatusNotFound, map[string]any{"message": "Not Found"})
	cs := connect(t, testOptions(t, f))

	assert.Contains(t, requireToolError(t, callTool(t, cs, "list_segments", map[string]any{})), "marketing turned on")
}

// jsonValue returns v as the fake API decodes a request body: numbers are
// float64.
func jsonValue(t *testing.T, v map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

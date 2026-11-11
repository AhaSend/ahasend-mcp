package ahasendmcp

import (
	"slices"
	"strings"
)

// scopeFamilies are the v2 scopes that take a sending domain. A family is
// met by "<family>:all" or by any "<family>:{domain}".
var scopeFamilies = []string{
	"messages:send",
	"messages:cancel",
	"messages:read",
	"domains:delete",
	"webhooks:read",
	"webhooks:write",
	"webhooks:delete",
	"routes:read",
	"routes:write",
	"routes:delete",
	"smtp-credentials:read",
	"smtp-credentials:write",
	"smtp-credentials:delete",
	"statistics:read",
}

// earlierFamilyNames maps each earlier name of a scope family to its current
// name. The API accepts both names, and they give the same access.
var earlierFamilyNames = map[string]string{
	"statistics-transactional:read": "statistics:read",
}

func isScopeFamily(scope string) bool {
	return slices.Contains(scopeFamilies, scope)
}

// hasScope reports whether scopes meet the required scope. A static scope
// is met only by itself; a scope family by "<family>:all" or by
// "<family>:{domain}", under its current name or an earlier name.
func hasScope(scopes []string, required string) bool {
	family := isScopeFamily(required)
	for _, s := range scopes {
		if s == required && !family {
			return true
		}
		if family && currentFamily(s) == required {
			return true
		}
	}
	return false
}

// currentFamily returns the current name of the family of a scope in the form
// "<family>:all" or "<family>:{domain}", and "" for a scope in neither form.
func currentFamily(scope string) string {
	family, _, found := strings.Cut(scope, ":{")
	if !found {
		family, found = strings.CutSuffix(scope, ":all")
	}
	if !found {
		return ""
	}
	if current, ok := earlierFamilyNames[family]; ok {
		return current
	}
	return family
}

// visible reports whether a tool is listed for these options: the key meets
// every required scope (or the scopes are unknown), the tool's toolset is
// chosen (when Toolsets is set), and the tool is read-only (when ReadOnly
// is set).
func visible(info ToolInfo, opts Options) bool {
	if opts.Scopes != nil {
		for _, required := range info.RequiredScopes {
			if !hasScope(opts.Scopes, required) {
				return false
			}
		}
	}
	if opts.Toolsets != nil && !slices.Contains(opts.Toolsets, info.Toolset) {
		return false
	}
	if opts.ReadOnly && !info.Annotations.ReadOnlyHint {
		return false
	}
	return true
}

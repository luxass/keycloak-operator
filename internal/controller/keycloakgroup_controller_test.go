package controller

import (
	"testing"

	"github.com/Hostzero-GmbH/keycloak-operator/internal/keycloak"
)

// TestFindTopLevelGroupByName documents that the helper only matches against
// the top-level entries of the supplied slice. Callers (the KeycloakGroup
// reconciler) are expected to scope the slice to the right parent before
// calling it. Recursing into SubGroups would be incorrect across parents and
// is also unreliable on Keycloak 23+, where SubGroups is empty in the
// realm-wide listing.
func TestFindTopLevelGroupByName(t *testing.T) {
	ptr := func(s string) *string { return &s }
	groups := []keycloak.GroupRepresentation{
		{Name: ptr("alpha")},
		{Name: ptr("beta"), SubGroups: []keycloak.GroupRepresentation{{Name: ptr("nested")}}},
	}

	if got := findTopLevelGroupByName(groups, "beta"); got == nil || got.Name == nil || *got.Name != "beta" {
		t.Errorf("findTopLevelGroupByName(beta) = %+v, want beta", got)
	}
	if got := findTopLevelGroupByName(groups, "missing"); got != nil {
		t.Errorf("findTopLevelGroupByName(missing) = %+v, want nil", got)
	}
	if got := findTopLevelGroupByName(groups, "nested"); got != nil {
		t.Errorf("findTopLevelGroupByName must not recurse into SubGroups, got %+v", got)
	}
}

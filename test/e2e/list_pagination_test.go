package e2e

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestListPaginationE2E exercises the listAll[T] helper added in
// fix/list-pagination against a real Keycloak instance. The Keycloak Admin
// API caps every list response at 100 items by default, so before this fix
// realms with more than 100 of any resource silently lost the rest. The
// regression we are guarding against is GetClients/GetUsers/GetGroups/
// GetRealmRoles returning at most 100 items.
//
// We bypass the operator and CRDs entirely here: we provision the resources
// directly via the Keycloak Admin API, then call the production list helpers
// and assert that we see *all* of them. Going through the operator would
// require creating hundreds of CRs, which is an order of magnitude slower
// and tests reconciler behavior, not the client pagination we care about.
//
// To keep the test cheap on CI we use the smallest count that still proves
// pagination — one full page (100) plus a handful — for each resource type.
func TestListPaginationE2E(t *testing.T) {
	skipIfNoCluster(t)
	skipIfNoKeycloakAccess(t)

	const overflow = 5 // items in the 2nd page; total per type = 100 + overflow
	const want = 100 + overflow

	instanceName, instanceNS := getOrCreateInstance(t)
	realmName := createTestRealm(t, instanceName, instanceNS, "pagination")
	kc := getInternalKeycloakClient(t)

	suffix := time.Now().UnixNano()

	t.Run("Users", func(t *testing.T) {
		prefix := fmt.Sprintf("page-user-%d-", suffix)
		for i := 0; i < want; i++ {
			name := fmt.Sprintf("%s%03d", prefix, i)
			body := json.RawMessage(fmt.Sprintf(`{"username":%q,"enabled":true}`, name))
			_, err := kc.CreateUser(ctx, realmName, body)
			require.NoErrorf(t, err, "create user %s", name)
		}

		users, err := kc.GetUsers(ctx, realmName, nil)
		require.NoError(t, err)
		got := countMatching(len(users), func(i int) string {
			return derefString(users[i].Username)
		}, prefix)
		assert.Equalf(t, want, got, "GetUsers must paginate past 100 (got %d, want %d)", got, want)
	})

	t.Run("Groups", func(t *testing.T) {
		prefix := fmt.Sprintf("page-group-%d-", suffix)
		for i := 0; i < want; i++ {
			name := fmt.Sprintf("%s%03d", prefix, i)
			body := json.RawMessage(fmt.Sprintf(`{"name":%q}`, name))
			_, err := kc.CreateGroup(ctx, realmName, body)
			require.NoErrorf(t, err, "create group %s", name)
		}

		groups, err := kc.GetGroups(ctx, realmName, nil)
		require.NoError(t, err)
		got := countMatching(len(groups), func(i int) string {
			return derefString(groups[i].Name)
		}, prefix)
		assert.Equalf(t, want, got, "GetGroups must paginate past 100 (got %d, want %d)", got, want)
	})

	t.Run("RealmRoles", func(t *testing.T) {
		prefix := fmt.Sprintf("page-role-%d-", suffix)
		for i := 0; i < want; i++ {
			name := fmt.Sprintf("%s%03d", prefix, i)
			body := json.RawMessage(fmt.Sprintf(`{"name":%q}`, name))
			_, err := kc.CreateRealmRole(ctx, realmName, body)
			require.NoErrorf(t, err, "create role %s", name)
		}

		roles, err := kc.GetRealmRoles(ctx, realmName)
		require.NoError(t, err)
		got := countMatching(len(roles), func(i int) string {
			return derefString(roles[i].Name)
		}, prefix)
		assert.Equalf(t, want, got, "GetRealmRoles must paginate past 100 (got %d, want %d)", got, want)
	})

	t.Run("Clients", func(t *testing.T) {
		prefix := fmt.Sprintf("page-client-%d-", suffix)
		for i := 0; i < want; i++ {
			name := fmt.Sprintf("%s%03d", prefix, i)
			body := json.RawMessage(fmt.Sprintf(`{"clientId":%q,"enabled":true,"protocol":"openid-connect"}`, name))
			_, err := kc.CreateClient(ctx, realmName, body)
			require.NoErrorf(t, err, "create client %s", name)
		}

		clients, err := kc.GetClients(ctx, realmName, nil)
		require.NoError(t, err)
		got := countMatching(len(clients), func(i int) string {
			return derefString(clients[i].ClientID)
		}, prefix)
		assert.Equalf(t, want, got, "GetClients must paginate past 100 (got %d, want %d)", got, want)
	})

	// Cross-check with the underlying RawMessage list helpers (used by the
	// exporter). These don't currently use listAll, so they're a useful
	// reference: with stock Keycloak they should hit the 100-item cap. If
	// this assumption ever changes (e.g. Keycloak's defaults change), we'll
	// notice here.
	t.Run("RawListsHit100Cap_DocumentingDefault", func(t *testing.T) {
		raw, err := kc.GetUsersRaw(ctx, realmName, nil)
		require.NoError(t, err)
		// We pre-created 100+overflow users; the raw helper does not
		// paginate, so it should still cap at 100.
		assert.LessOrEqualf(t, len(raw), 100,
			"GetUsersRaw is expected to hit Keycloak's default page cap; if this fails, "+
				"either pagination was added to it (great — update this test) or Keycloak's "+
				"defaults changed (got %d items)", len(raw))
	})

}

func countMatching(n int, at func(int) string, prefix string) int {
	c := 0
	for i := 0; i < n; i++ {
		if strings.HasPrefix(at(i), prefix) {
			c++
		}
	}
	return c
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

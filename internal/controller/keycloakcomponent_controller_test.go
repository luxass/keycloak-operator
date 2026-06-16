package controller

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Hostzero-GmbH/keycloak-operator/internal/keycloak"
)

func TestFindMatchingComponentIDAdoptsUnnamedUserProfileComponent(t *testing.T) {
	componentID := "existing-user-profile-component"
	providerID := declarativeUserProfileProviderID
	providerType := userProfileProviderType
	parentID := "realm-id"

	components := []keycloak.ComponentRepresentation{
		{
			ID:           &componentID,
			ProviderID:   &providerID,
			ProviderType: &providerType,
			ParentID:     &parentID,
			// Keycloak-created user-profile components can be unnamed when they
			// are created through the /users/profile Admin API or User Profile UI.
			Name: nil,
		},
	}

	got, err := findMatchingComponentID(components, componentIdentity{
		Name:         declarativeUserProfileProviderID,
		ProviderID:   declarativeUserProfileProviderID,
		ProviderType: userProfileProviderType,
		ParentID:     parentID,
	})
	require.NoError(t, err)
	require.Equal(t, componentID, got)
}

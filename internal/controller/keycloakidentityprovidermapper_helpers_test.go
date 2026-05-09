package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	keycloakv1beta1 "github.com/Hostzero-GmbH/keycloak-operator/api/v1beta1"
)

func TestIdentityProviderAlias(t *testing.T) {
	t.Run("alias from definition", func(t *testing.T) {
		idp := &keycloakv1beta1.KeycloakIdentityProvider{
			ObjectMeta: metav1.ObjectMeta{Name: "fallback"},
			Spec: keycloakv1beta1.KeycloakIdentityProviderSpec{
				Definition: runtime.RawExtension{Raw: []byte(`{"alias":"oidc","providerId":"oidc"}`)},
			},
		}
		alias, err := identityProviderAlias(idp)
		require.NoError(t, err)
		assert.Equal(t, "oidc", alias)
	})

	t.Run("falls back to metadata.name when alias missing", func(t *testing.T) {
		idp := &keycloakv1beta1.KeycloakIdentityProvider{
			ObjectMeta: metav1.ObjectMeta{Name: "github"},
			Spec: keycloakv1beta1.KeycloakIdentityProviderSpec{
				Definition: runtime.RawExtension{Raw: []byte(`{"providerId":"github"}`)},
			},
		}
		alias, err := identityProviderAlias(idp)
		require.NoError(t, err)
		assert.Equal(t, "github", alias)
	})

	t.Run("falls back to metadata.name when alias empty string", func(t *testing.T) {
		idp := &keycloakv1beta1.KeycloakIdentityProvider{
			ObjectMeta: metav1.ObjectMeta{Name: "saml-prod"},
			Spec: keycloakv1beta1.KeycloakIdentityProviderSpec{
				Definition: runtime.RawExtension{Raw: []byte(`{"alias":"","providerId":"saml"}`)},
			},
		}
		alias, err := identityProviderAlias(idp)
		require.NoError(t, err)
		assert.Equal(t, "saml-prod", alias)
	})

	t.Run("falls back to metadata.name on empty definition", func(t *testing.T) {
		idp := &keycloakv1beta1.KeycloakIdentityProvider{
			ObjectMeta: metav1.ObjectMeta{Name: "empty"},
		}
		alias, err := identityProviderAlias(idp)
		require.NoError(t, err)
		assert.Equal(t, "empty", alias)
	})

	t.Run("returns error on invalid JSON", func(t *testing.T) {
		idp := &keycloakv1beta1.KeycloakIdentityProvider{
			ObjectMeta: metav1.ObjectMeta{Name: "broken"},
			Spec: keycloakv1beta1.KeycloakIdentityProviderSpec{
				Definition: runtime.RawExtension{Raw: []byte(`not-json`)},
			},
		}
		_, err := identityProviderAlias(idp)
		require.Error(t, err)
	})
}

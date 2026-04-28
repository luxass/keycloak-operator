package controller

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/Hostzero-GmbH/keycloak-operator/internal/keycloak"
)

func TestFilterTopLevelExecutions(t *testing.T) {
	zero := 0
	one := 1

	tests := []struct {
		name     string
		input    []keycloak.AuthenticationExecutionInfo
		expected int
	}{
		{
			name:     "empty list",
			input:    nil,
			expected: 0,
		},
		{
			name: "all level 0",
			input: []keycloak.AuthenticationExecutionInfo{
				{Level: &zero},
				{Level: &zero},
			},
			expected: 2,
		},
		{
			name: "mixed levels",
			input: []keycloak.AuthenticationExecutionInfo{
				{Level: &zero},
				{Level: &one},
				{Level: &zero},
				{Level: &one},
			},
			expected: 2,
		},
		{
			name: "no level info returns all",
			input: []keycloak.AuthenticationExecutionInfo{
				{},
				{},
			},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterTopLevelExecutions(tt.input)
			require.Len(t, result, tt.expected)
		})
	}
}

func TestMatchesIdentifier(t *testing.T) {
	trueVal := true
	falseVal := false
	cookieProvider := "auth-cookie"
	formsAlias := "my-forms"

	tests := []struct {
		name     string
		exec     keycloak.AuthenticationExecutionInfo
		id       execIdentifier
		expected bool
	}{
		{
			name: "matches authenticator by provider ID",
			exec: keycloak.AuthenticationExecutionInfo{
				ProviderID:         &cookieProvider,
				AuthenticationFlow: &falseVal,
			},
			id:       execIdentifier{name: "auth-cookie", isFlow: false},
			expected: true,
		},
		{
			name: "does not match different provider",
			exec: keycloak.AuthenticationExecutionInfo{
				ProviderID:         &cookieProvider,
				AuthenticationFlow: &falseVal,
			},
			id:       execIdentifier{name: "auth-otp", isFlow: false},
			expected: false,
		},
		{
			name: "matches sub-flow by display name",
			exec: keycloak.AuthenticationExecutionInfo{
				DisplayName:        &formsAlias,
				AuthenticationFlow: &trueVal,
			},
			id:       execIdentifier{name: "my-forms", isFlow: true},
			expected: true,
		},
		{
			name: "does not match flow when looking for authenticator",
			exec: keycloak.AuthenticationExecutionInfo{
				DisplayName:        &formsAlias,
				AuthenticationFlow: &trueVal,
			},
			id:       execIdentifier{name: "my-forms", isFlow: false},
			expected: false,
		},
		{
			name: "does not match authenticator when looking for flow",
			exec: keycloak.AuthenticationExecutionInfo{
				ProviderID:         &cookieProvider,
				AuthenticationFlow: &falseVal,
			},
			id:       execIdentifier{name: "auth-cookie", isFlow: true},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesIdentifier(tt.exec, tt.id)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestParseExecutionsAcceptsBothShapes(t *testing.T) {
	t.Run("inline executions inside subFlow", func(t *testing.T) {
		raw := rawExt(t, `[
			{"authenticator":"auth-cookie","requirement":"ALTERNATIVE"},
			{
				"subFlow":{"alias":"forms","providerId":"basic-flow","executions":[
					{"authenticator":"auth-username-password-form","requirement":"REQUIRED"}
				]},
				"requirement":"ALTERNATIVE"
			}
		]`)
		execs, err := parseExecutions(raw)
		require.NoError(t, err)
		require.Len(t, execs, 2)
		require.Equal(t, "auth-cookie", execs[0].Authenticator)
		require.NotNil(t, execs[1].SubFlow)
		children := execs[1].children()
		require.Len(t, children, 1)
		require.Equal(t, "auth-username-password-form", children[0].Authenticator)
	})

	t.Run("sibling executions next to subFlow", func(t *testing.T) {
		raw := rawExt(t, `[
			{
				"subFlow":{"alias":"registration-form","providerId":"form-flow"},
				"requirement":"REQUIRED",
				"executions":[
					{"authenticator":"registration-user-creation","requirement":"REQUIRED"},
					{"authenticator":"registration-password-action","requirement":"REQUIRED"}
				]
			}
		]`)
		execs, err := parseExecutions(raw)
		require.NoError(t, err)
		require.Len(t, execs, 1)
		children := execs[0].children()
		require.Len(t, children, 2)
		require.Equal(t, "registration-user-creation", children[0].Authenticator)
		require.Equal(t, "registration-password-action", children[1].Authenticator)
	})

	t.Run("inline and sibling merge in declared order", func(t *testing.T) {
		raw := rawExt(t, `[
			{
				"subFlow":{"alias":"sub","providerId":"basic-flow","executions":[
					{"authenticator":"inside","requirement":"REQUIRED"}
				]},
				"requirement":"ALTERNATIVE",
				"executions":[
					{"authenticator":"sibling","requirement":"ALTERNATIVE"}
				]
			}
		]`)
		execs, err := parseExecutions(raw)
		require.NoError(t, err)
		children := execs[0].children()
		require.Len(t, children, 2)
		require.Equal(t, "inside", children[0].Authenticator)
		require.Equal(t, "sibling", children[1].Authenticator)
	})

	t.Run("arbitrary nesting depth", func(t *testing.T) {
		raw := rawExt(t, `[
			{"subFlow":{"alias":"l1","providerId":"basic-flow","executions":[
				{"subFlow":{"alias":"l2","providerId":"basic-flow","executions":[
					{"subFlow":{"alias":"l3","providerId":"basic-flow","executions":[
						{"subFlow":{"alias":"l4","providerId":"basic-flow","executions":[
							{"authenticator":"deep","requirement":"REQUIRED"}
						]},"requirement":"REQUIRED"}
					]},"requirement":"REQUIRED"}
				]},"requirement":"REQUIRED"}
			]},"requirement":"REQUIRED"}
		]`)
		execs, err := parseExecutions(raw)
		require.NoError(t, err)
		// Walk five levels down and verify the deepest authenticator survived.
		cur := execs[0]
		for depth := 1; depth <= 4; depth++ {
			children := cur.children()
			require.Len(t, children, 1, "depth %d", depth)
			cur = children[0]
		}
		require.Equal(t, "deep", cur.Authenticator)
	})
}

func TestParseExecutionsRejectsInvalidShape(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{
			name: "neither authenticator nor subFlow",
			json: `[{"requirement":"REQUIRED"}]`,
			want: "exactly one of authenticator or subFlow",
		},
		{
			name: "both authenticator and subFlow",
			json: `[{"authenticator":"x","subFlow":{"alias":"a","providerId":"basic-flow"},"requirement":"REQUIRED"}]`,
			want: "exactly one of authenticator or subFlow",
		},
		{
			name: "missing requirement",
			json: `[{"authenticator":"x"}]`,
			want: "requirement is required",
		},
		{
			name: "invalid requirement",
			json: `[{"authenticator":"x","requirement":"MAYBE"}]`,
			want: "is not one of",
		},
		{
			name: "subflow without alias",
			json: `[{"subFlow":{"providerId":"basic-flow"},"requirement":"REQUIRED"}]`,
			want: "subFlow.alias is required",
		},
		{
			name: "subflow without providerId",
			json: `[{"subFlow":{"alias":"a"},"requirement":"REQUIRED"}]`,
			want: "subFlow.providerId is required",
		},
		{
			name: "nested validation surfaces path",
			json: `[{"subFlow":{"alias":"a","providerId":"basic-flow","executions":[{"requirement":"REQUIRED"}]},"requirement":"REQUIRED"}]`,
			want: "[0].executions[0]",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseExecutions(rawExt(t, tc.json))
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestParseExecutionsEmpty(t *testing.T) {
	got, err := parseExecutions(runtime.RawExtension{})
	require.NoError(t, err)
	require.Nil(t, got)
}

func rawExt(t *testing.T, s string) runtime.RawExtension {
	t.Helper()
	// Round-trip through json to normalise whitespace and surface syntax errors here.
	var v interface{}
	require.NoError(t, json.Unmarshal([]byte(s), &v))
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return runtime.RawExtension{Raw: b}
}

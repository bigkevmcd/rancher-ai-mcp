package rbac

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rancher/rancher-ai-mcp/internal/middleware"
	"github.com/rancher/rancher-ai-mcp/pkg/client"
	"github.com/rancher/rancher-ai-mcp/pkg/client/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
)

func TestListGlobalRoleBindings(t *testing.T) {
	grb1 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion":     "management.cattle.io/v3",
			"kind":           "GlobalRoleBinding",
			"metadata":       map[string]any{"name": "grb-1"},
			"globalRoleName": "admin",
			"userName":       "u-user1",
		},
	}
	grb2 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion":     "management.cattle.io/v3",
			"kind":           "GlobalRoleBinding",
			"metadata":       map[string]any{"name": "grb-2"},
			"globalRoleName": "user",
			"userName":       "u-user2",
		},
	}
	grb3 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion":         "management.cattle.io/v3",
			"kind":               "GlobalRoleBinding",
			"metadata":           map[string]any{"name": "grb-3"},
			"globalRoleName":     "user",
			"groupPrincipalName": "github_team://1234",
		},
	}

	grb1JSON := `{"apiVersion": "management.cattle.io/v3", "kind": "GlobalRoleBinding", "metadata": {"name": "grb-1"}, "globalRoleName": "admin", "userName": "u-user1"}`
	grb2JSON := `{"apiVersion": "management.cattle.io/v3", "kind": "GlobalRoleBinding", "metadata": {"name": "grb-2"}, "globalRoleName": "user", "userName": "u-user2"}`
	grb3JSON := `{"apiVersion": "management.cattle.io/v3", "kind": "GlobalRoleBinding", "metadata": {"name": "grb-3"}, "globalRoleName": "user", "groupPrincipalName": "github_team://1234"}`
	uiContext := func(name string) string {
		return `{"cluster": "local", "kind": "GlobalRoleBinding", "name": "` + name + `", "namespace": "", "type": "globalrolebinding"}`
	}

	tests := map[string]struct {
		params         listGRBParams
		objects        []runtime.Object
		expectedResult string
		expectedError  string
	}{
		"list all GRBs": {
			params:         listGRBParams{},
			objects:        []runtime.Object{grb1, grb2, grb3},
			expectedResult: `{"llm": [` + grb1JSON + `,` + grb2JSON + `,` + grb3JSON + `], "uiContext": [` + uiContext("grb-1") + `,` + uiContext("grb-2") + `,` + uiContext("grb-3") + `]}`,
		},
		"filter by user includes bindings for the user's groups": {
			params:         listGRBParams{User: "u-user1"},
			objects:        []runtime.Object{grb1, grb2, grb3},
			expectedResult: `{"llm": {"resources": [` + grb1JSON + `,` + grb3JSON + `], "note": "includes bindings for the groups the user belongs to: github_team://1234"}, "uiContext": [` + uiContext("grb-1") + `,` + uiContext("grb-3") + `]}`,
		},
		"filter by username for a user without groups": {
			params:         listGRBParams{User: "user2"},
			objects:        []runtime.Object{grb1, grb2, grb3},
			expectedResult: `{"llm": [` + grb2JSON + `], "uiContext": [` + uiContext("grb-2") + `]}`,
		},
		"filter by user and global role": {
			params:         listGRBParams{User: "user1", GlobalRole: "user"},
			objects:        []runtime.Object{grb1, grb2, grb3},
			expectedResult: `{"llm": {"resources": [` + grb3JSON + `], "note": "includes bindings for the groups the user belongs to: github_team://1234"}, "uiContext": [` + uiContext("grb-3") + `]}`,
		},
		"filter by group": {
			params:         listGRBParams{Group: "github_team://1234"},
			objects:        []runtime.Object{grb1, grb2, grb3},
			expectedResult: `{"llm": [` + grb3JSON + `], "uiContext": [` + uiContext("grb-3") + `]}`,
		},
		"filter by global role": {
			params:         listGRBParams{GlobalRole: "user"},
			objects:        []runtime.Object{grb1, grb2, grb3},
			expectedResult: `{"llm": [` + grb2JSON + `,` + grb3JSON + `], "uiContext": [` + uiContext("grb-2") + `,` + uiContext("grb-3") + `]}`,
		},
		"user and group cannot both be specified": {
			params:        listGRBParams{User: "u-user1", Group: "github_team://1234"},
			objects:       []runtime.Object{grb1, grb2, grb3},
			expectedError: "only one of user or group can be specified",
		},
		"unknown user": {
			params:        listGRBParams{User: "nobody"},
			objects:       []runtime.Object{grb1, grb2, grb3},
			expectedError: `user "nobody" not found`,
		},
		"no GRBs found": {
			params:         listGRBParams{},
			objects:        []runtime.Object{},
			expectedResult: `{"llm": "no resources found"}`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			fakeDynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(rbacScheme(), rbacGVRs, append(tt.objects, testUsers()...)...)
			c := &client.Client{
				DynClientCreator: func(_ *rest.Config) (dynamic.Interface, error) { return fakeDynClient, nil },
			}
			tools := NewTools(test.WrapClient(c, fakeToken), false)

			result, _, err := tools.listGlobalRoleBindings(
				middleware.WithToken(t.Context(), fakeToken),
				test.NewCallToolRequest(fakeURL),
				tt.params,
			)
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			assert.JSONEq(t, tt.expectedResult, result.Content[0].(*mcp.TextContent).Text)
		})
	}
}

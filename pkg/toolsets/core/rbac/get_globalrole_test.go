package rbac

import (
	"errors"
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
	k8stesting "k8s.io/client-go/testing"
)

func TestGetGlobalRole(t *testing.T) {
	admin := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion":  "management.cattle.io/v3",
			"kind":        "GlobalRole",
			"metadata":    map[string]any{"name": "admin"},
			"displayName": "Admin",
			"rules": []any{
				map[string]any{"verbs": []any{"*"}, "resources": []any{"*"}, "apiGroups": []any{"*"}},
			},
		},
	}
	nodeDrivers := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion":  "management.cattle.io/v3",
			"kind":        "GlobalRole",
			"metadata":    map[string]any{"name": "nodedrivers-manage"},
			"displayName": "Manage Node Drivers",
		},
	}
	duplicate1 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion":  "management.cattle.io/v3",
			"kind":        "GlobalRole",
			"metadata":    map[string]any{"name": "gr-aaaaa"},
			"displayName": "Test Role",
		},
	}
	duplicate2 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion":  "management.cattle.io/v3",
			"kind":        "GlobalRole",
			"metadata":    map[string]any{"name": "gr-bbbbb"},
			"displayName": "Test Role",
		},
	}
	noDisplayName := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "management.cattle.io/v3",
			"kind":       "GlobalRole",
			"metadata":   map[string]any{"name": "gr-nodisplay"},
		},
	}

	tests := map[string]struct {
		params         getGlobalRoleParams
		objects        []runtime.Object
		reactorErr     error
		expectedError  string
		expectedResult string
	}{
		"get global role by name": {
			params:  getGlobalRoleParams{Name: "admin"},
			objects: []runtime.Object{admin, nodeDrivers},
			expectedResult: `{
				"llm": [
					{
						"apiVersion": "management.cattle.io/v3",
						"kind": "GlobalRole",
						"metadata": {"name": "admin"},
						"displayName": "Admin",
						"rules": [{"verbs": ["*"], "resources": ["*"], "apiGroups": ["*"]}]
					}
				],
				"uiContext": [
					{"cluster": "local", "kind": "GlobalRole", "name": "admin", "namespace": "", "type": "globalrole"}
				]
			}`,
		},
		"get global role by display name ignoring case": {
			params:  getGlobalRoleParams{Name: "manage node drivers"},
			objects: []runtime.Object{admin, nodeDrivers},
			expectedResult: `{
				"llm": [
					{"apiVersion": "management.cattle.io/v3", "kind": "GlobalRole", "metadata": {"name": "nodedrivers-manage"}, "displayName": "Manage Node Drivers"}
				],
				"uiContext": [
					{"cluster": "local", "kind": "GlobalRole", "name": "nodedrivers-manage", "namespace": "", "type": "globalrole"}
				]
			}`,
		},
		"multiple global roles with the display name": {
			params:  getGlobalRoleParams{Name: "Test Role"},
			objects: []runtime.Object{admin, duplicate1, duplicate2},
			expectedResult: `{
				"llm": {
					"resources": [
						{"apiVersion": "management.cattle.io/v3", "kind": "GlobalRole", "metadata": {"name": "gr-aaaaa"}, "displayName": "Test Role"},
						{"apiVersion": "management.cattle.io/v3", "kind": "GlobalRole", "metadata": {"name": "gr-bbbbb"}, "displayName": "Test Role"}
					],
					"note": "multiple global roles have this display name, ask the user which one they mean"
				},
				"uiContext": [
					{"cluster": "local", "kind": "GlobalRole", "name": "gr-aaaaa", "namespace": "", "type": "globalrole"},
					{"cluster": "local", "kind": "GlobalRole", "name": "gr-bbbbb", "namespace": "", "type": "globalrole"}
				]
			}`,
		},
		"no match returns the available global roles": {
			params:  getGlobalRoleParams{Name: "Configure Node Drivers"},
			objects: []runtime.Object{admin, nodeDrivers, noDisplayName},
			expectedResult: `{
				"llm": {
					"note": "no global role has the name or display name \"Configure Node Drivers\". Suggest the closest of the available global roles to the user",
					"availableGlobalRoles": [
						{"name": "admin", "displayName": "Admin"},
						{"name": "gr-nodisplay"},
						{"name": "nodedrivers-manage", "displayName": "Manage Node Drivers"}
					]
				}
			}`,
		},
		"errors other than not found are returned": {
			params:        getGlobalRoleParams{Name: "admin"},
			objects:       []runtime.Object{admin},
			reactorErr:    errors.New("connection refused"),
			expectedError: "connection refused",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			fakeDynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(rbacScheme(), rbacGVRs, tt.objects...)
			if tt.reactorErr != nil {
				fakeDynClient.PrependReactor("get", "globalroles", func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, tt.reactorErr
				})
			}
			c := &client.Client{
				DynClientCreator: func(_ *rest.Config) (dynamic.Interface, error) { return fakeDynClient, nil },
			}
			tools := NewTools(test.WrapClient(c, fakeToken), false)

			result, _, err := tools.getGlobalRole(
				middleware.WithToken(t.Context(), fakeToken),
				test.NewCallToolRequest(fakeURL),
				tt.params,
			)
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
			} else {
				require.NoError(t, err)
				assert.JSONEq(t, tt.expectedResult, result.Content[0].(*mcp.TextContent).Text)
			}
		})
	}
}

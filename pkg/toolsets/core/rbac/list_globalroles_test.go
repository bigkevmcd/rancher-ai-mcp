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

func TestListGlobalRoles(t *testing.T) {
	gr1 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "management.cattle.io/v3",
			"kind":       "GlobalRole",
			"metadata":   map[string]any{"name": "admin"},
			"builtin":    true,
		},
	}
	gr2 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion":  "management.cattle.io/v3",
			"kind":        "GlobalRole",
			"metadata":    map[string]any{"name": "gr-custom"},
			"displayName": "Custom Role",
		},
	}

	tests := map[string]struct {
		objects        []runtime.Object
		expectedResult string
	}{
		"list all global roles": {
			objects: []runtime.Object{gr1, gr2},
			expectedResult: `{
				"llm": [
					{"apiVersion": "management.cattle.io/v3", "kind": "GlobalRole", "metadata": {"name": "admin"}, "builtin": true},
					{"apiVersion": "management.cattle.io/v3", "kind": "GlobalRole", "metadata": {"name": "gr-custom"}, "displayName": "Custom Role"}
				],
				"uiContext": [
					{"cluster": "local", "kind": "GlobalRole", "name": "admin", "namespace": "", "type": "globalrole"},
					{"cluster": "local", "kind": "GlobalRole", "name": "gr-custom", "namespace": "", "type": "globalrole"}
				]
			}`,
		},
		"no global roles found": {
			objects:        []runtime.Object{},
			expectedResult: `{"llm": "no resources found"}`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			fakeDynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(rbacScheme(), rbacGVRs, tt.objects...)
			c := &client.Client{
				DynClientCreator: func(_ *rest.Config) (dynamic.Interface, error) { return fakeDynClient, nil },
			}
			tools := NewTools(test.WrapClient(c, fakeToken), false)

			result, _, err := tools.listGlobalRoles(
				middleware.WithToken(t.Context(), fakeToken),
				test.NewCallToolRequest(fakeURL),
				struct{}{},
			)
			require.NoError(t, err)
			assert.JSONEq(t, tt.expectedResult, result.Content[0].(*mcp.TextContent).Text)
		})
	}
}

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

func TestListClusterRoleTemplateBindings(t *testing.T) {
	crtb1 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "management.cattle.io/v3",
			"kind":       "ClusterRoleTemplateBinding",
			"metadata": map[string]any{
				"name":      "crtb-1",
				"namespace": "local",
			},
			"clusterName":      "local",
			"userName":         "u-user1",
			"roleTemplateName": "cluster-owner",
		},
	}
	crtb2 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "management.cattle.io/v3",
			"kind":       "ClusterRoleTemplateBinding",
			"metadata": map[string]any{
				"name":      "crtb-2",
				"namespace": "local",
			},
			"clusterName":      "local",
			"userName":         "u-user2",
			"roleTemplateName": "cluster-member",
		},
	}
	crtb3 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "management.cattle.io/v3",
			"kind":       "ClusterRoleTemplateBinding",
			"metadata": map[string]any{
				"name":      "crtb-3",
				"namespace": "downstream",
			},
			"clusterName":      "downstream",
			"userName":         "u-user1",
			"roleTemplateName": "cluster-member",
		},
	}

	crtbGroup := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "management.cattle.io/v3",
			"kind":       "ClusterRoleTemplateBinding",
			"metadata": map[string]any{
				"name":      "crtb-group",
				"namespace": "local",
			},
			"clusterName":        "local",
			"groupPrincipalName": "github_team://1234",
			"roleTemplateName":   "cluster-member",
		},
	}
	crtbLegacyGroup := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "management.cattle.io/v3",
			"kind":       "ClusterRoleTemplateBinding",
			"metadata": map[string]any{
				"name":      "crtb-legacy-group",
				"namespace": "local",
			},
			"clusterName":      "local",
			"groupName":        "github_team://1234",
			"roleTemplateName": "cluster-owner",
		},
	}

	tests := map[string]struct {
		params         listCRTBParams
		objects        []runtime.Object
		expectedResult string
		expectedError  string
	}{
		"list all CRTBs for a cluster": {
			params:  listCRTBParams{Cluster: "local"},
			objects: []runtime.Object{crtb1, crtb2, crtb3},
			expectedResult: `{
				"llm": [
					{
						"apiVersion": "management.cattle.io/v3",
						"kind": "ClusterRoleTemplateBinding",
						"metadata": {"name": "crtb-1", "namespace": "local"},
						"clusterName": "local",
						"roleTemplateName": "cluster-owner",
						"userName": "u-user1"
					},
					{
						"apiVersion": "management.cattle.io/v3",
						"kind": "ClusterRoleTemplateBinding",
						"metadata": {"name": "crtb-2", "namespace": "local"},
						"clusterName": "local",
						"roleTemplateName": "cluster-member",
						"userName": "u-user2"
					}
				],
				"uiContext": [
					{"cluster": "local", "kind": "ClusterRoleTemplateBinding", "name": "crtb-1", "namespace": "local", "type": "clusterroletemplatebinding"},
					{"cluster": "local", "kind": "ClusterRoleTemplateBinding", "name": "crtb-2", "namespace": "local", "type": "clusterroletemplatebinding"}
				]
			}`,
		},
		"filter by user": {
			params:  listCRTBParams{Cluster: "local", User: "u-user1"},
			objects: []runtime.Object{crtb1, crtb2, crtb3},
			expectedResult: `{
				"llm": {
					"resources": [
						{
							"apiVersion": "management.cattle.io/v3",
							"kind": "ClusterRoleTemplateBinding",
							"metadata": {"name": "crtb-1", "namespace": "local"},
							"clusterName": "local",
							"roleTemplateName": "cluster-owner",
							"userName": "u-user1"
						}
					],
					"note": "includes bindings for the groups the user belongs to: github_team://1234"
				},
				"uiContext": [
					{"cluster": "local", "kind": "ClusterRoleTemplateBinding", "name": "crtb-1", "namespace": "local", "type": "clusterroletemplatebinding"}
				]
			}`,
		},
		"filter by username includes bindings for the user's groups": {
			params:  listCRTBParams{Cluster: "local", User: "user1"},
			objects: []runtime.Object{crtb1, crtb2, crtbGroup},
			expectedResult: `{
				"llm": {
					"resources": [
						{
							"apiVersion": "management.cattle.io/v3",
							"kind": "ClusterRoleTemplateBinding",
							"metadata": {"name": "crtb-1", "namespace": "local"},
							"clusterName": "local",
							"roleTemplateName": "cluster-owner",
							"userName": "u-user1"
						},
						{
							"apiVersion": "management.cattle.io/v3",
							"kind": "ClusterRoleTemplateBinding",
							"metadata": {"name": "crtb-group", "namespace": "local"},
							"clusterName": "local",
							"groupPrincipalName": "github_team://1234",
							"roleTemplateName": "cluster-member"
						}
					],
					"note": "includes bindings for the groups the user belongs to: github_team://1234"
				},
				"uiContext": [
					{"cluster": "local", "kind": "ClusterRoleTemplateBinding", "name": "crtb-1", "namespace": "local", "type": "clusterroletemplatebinding"},
					{"cluster": "local", "kind": "ClusterRoleTemplateBinding", "name": "crtb-group", "namespace": "local", "type": "clusterroletemplatebinding"}
				]
			}`,
		},
		"filter by group matches groupPrincipalName and groupName": {
			params:  listCRTBParams{Cluster: "local", Group: "github_team://1234"},
			objects: []runtime.Object{crtb1, crtbGroup, crtbLegacyGroup},
			expectedResult: `{
				"llm": [
					{
						"apiVersion": "management.cattle.io/v3",
						"kind": "ClusterRoleTemplateBinding",
						"metadata": {"name": "crtb-group", "namespace": "local"},
						"clusterName": "local",
						"groupPrincipalName": "github_team://1234",
						"roleTemplateName": "cluster-member"
					},
					{
						"apiVersion": "management.cattle.io/v3",
						"kind": "ClusterRoleTemplateBinding",
						"metadata": {"name": "crtb-legacy-group", "namespace": "local"},
						"clusterName": "local",
						"groupName": "github_team://1234",
						"roleTemplateName": "cluster-owner"
					}
				],
				"uiContext": [
					{"cluster": "local", "kind": "ClusterRoleTemplateBinding", "name": "crtb-group", "namespace": "local", "type": "clusterroletemplatebinding"},
					{"cluster": "local", "kind": "ClusterRoleTemplateBinding", "name": "crtb-legacy-group", "namespace": "local", "type": "clusterroletemplatebinding"}
				]
			}`,
		},
		"user and group cannot both be specified": {
			params:        listCRTBParams{Cluster: "local", User: "u-user1", Group: "github_team://1234"},
			objects:       []runtime.Object{crtb1},
			expectedError: "only one of user or group can be specified",
		},
		"no CRTBs found": {
			params:         listCRTBParams{Cluster: "local"},
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

			result, _, err := tools.listClusterRoleTemplateBindings(
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

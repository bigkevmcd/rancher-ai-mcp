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

func TestListProjectRoleTemplateBindings(t *testing.T) {
	prtb1 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "management.cattle.io/v3",
			"kind":       "ProjectRoleTemplateBinding",
			"metadata": map[string]any{
				"name":      "prtb-1",
				"namespace": "p-abc",
			},
			"projectName":      "local:p-abc",
			"userName":         "u-user1",
			"roleTemplateName": "project-owner",
		},
	}
	prtb2 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "management.cattle.io/v3",
			"kind":       "ProjectRoleTemplateBinding",
			"metadata": map[string]any{
				"name":      "prtb-2",
				"namespace": "p-abc",
			},
			"projectName":      "local:p-abc",
			"userName":         "u-user2",
			"roleTemplateName": "project-member",
		},
	}
	prtb3 := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "management.cattle.io/v3",
			"kind":       "ProjectRoleTemplateBinding",
			"metadata": map[string]any{
				"name":      "prtb-3",
				"namespace": "local-p-xyz",
			},
			"projectName":      "local:p-xyz",
			"userName":         "u-user1",
			"roleTemplateName": "project-member",
		},
	}
	projectABC := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "management.cattle.io/v3",
			"kind":       "Project",
			"metadata": map[string]any{
				"name":      "p-abc",
				"namespace": "local",
			},
			"status": map[string]any{
				"backingNamespace": "p-abc",
			},
		},
	}

	prtbGroup := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "management.cattle.io/v3",
			"kind":       "ProjectRoleTemplateBinding",
			"metadata": map[string]any{
				"name":      "prtb-group",
				"namespace": "local-p-xyz",
			},
			"projectName":        "local:p-xyz",
			"groupPrincipalName": "github_team://1234",
			"roleTemplateName":   "project-member",
		},
	}

	tests := map[string]struct {
		params         listPRTBParams
		objects        []runtime.Object
		expectedResult string
		expectedError  string
	}{
		"list all PRTBs for a cluster": {
			params:  listPRTBParams{Cluster: "local"},
			objects: []runtime.Object{prtb1, prtb2, prtb3},
			expectedResult: `{
				"llm": [
					{
						"apiVersion": "management.cattle.io/v3",
						"kind": "ProjectRoleTemplateBinding",
						"metadata": {"name": "prtb-3", "namespace": "local-p-xyz"},
						"projectName": "local:p-xyz",
						"roleTemplateName": "project-member",
						"userName": "u-user1"
					},
					{
						"apiVersion": "management.cattle.io/v3",
						"kind": "ProjectRoleTemplateBinding",
						"metadata": {"name": "prtb-1", "namespace": "p-abc"},
						"projectName": "local:p-abc",
						"roleTemplateName": "project-owner",
						"userName": "u-user1"
					},
					{
						"apiVersion": "management.cattle.io/v3",
						"kind": "ProjectRoleTemplateBinding",
						"metadata": {"name": "prtb-2", "namespace": "p-abc"},
						"projectName": "local:p-abc",
						"roleTemplateName": "project-member",
						"userName": "u-user2"
					}
				],
				"uiContext": [
					{"cluster": "local", "kind": "ProjectRoleTemplateBinding", "name": "prtb-3", "namespace": "local-p-xyz", "type": "projectroletemplatebinding"},
					{"cluster": "local", "kind": "ProjectRoleTemplateBinding", "name": "prtb-1", "namespace": "p-abc", "type": "projectroletemplatebinding"},
					{"cluster": "local", "kind": "ProjectRoleTemplateBinding", "name": "prtb-2", "namespace": "p-abc", "type": "projectroletemplatebinding"}
				]
			}`,
		},
		"filter by project": {
			params:  listPRTBParams{Cluster: "local", ProjectID: "p-abc"},
			objects: []runtime.Object{projectABC, prtb1, prtb2, prtb3},
			expectedResult: `{
				"llm": [
					{
						"apiVersion": "management.cattle.io/v3",
						"kind": "ProjectRoleTemplateBinding",
						"metadata": {"name": "prtb-1", "namespace": "p-abc"},
						"projectName": "local:p-abc",
						"roleTemplateName": "project-owner",
						"userName": "u-user1"
					},
					{
						"apiVersion": "management.cattle.io/v3",
						"kind": "ProjectRoleTemplateBinding",
						"metadata": {"name": "prtb-2", "namespace": "p-abc"},
						"projectName": "local:p-abc",
						"roleTemplateName": "project-member",
						"userName": "u-user2"
					}
				],
				"uiContext": [
					{"cluster": "local", "kind": "ProjectRoleTemplateBinding", "name": "prtb-1", "namespace": "p-abc", "type": "projectroletemplatebinding"},
					{"cluster": "local", "kind": "ProjectRoleTemplateBinding", "name": "prtb-2", "namespace": "p-abc", "type": "projectroletemplatebinding"}
				]
			}`,
		},
		"filter by user includes bindings for the user's groups": {
			params:  listPRTBParams{Cluster: "local", User: "u-user1"},
			objects: []runtime.Object{prtb1, prtb2, prtb3, prtbGroup},
			expectedResult: `{
				"llm": {
					"resources": [
						{
							"apiVersion": "management.cattle.io/v3",
							"kind": "ProjectRoleTemplateBinding",
							"metadata": {"name": "prtb-3", "namespace": "local-p-xyz"},
							"projectName": "local:p-xyz",
							"roleTemplateName": "project-member",
							"userName": "u-user1"
						},
						{
							"apiVersion": "management.cattle.io/v3",
							"kind": "ProjectRoleTemplateBinding",
							"metadata": {"name": "prtb-group", "namespace": "local-p-xyz"},
							"projectName": "local:p-xyz",
							"groupPrincipalName": "github_team://1234",
							"roleTemplateName": "project-member"
						},
						{
							"apiVersion": "management.cattle.io/v3",
							"kind": "ProjectRoleTemplateBinding",
							"metadata": {"name": "prtb-1", "namespace": "p-abc"},
							"projectName": "local:p-abc",
							"roleTemplateName": "project-owner",
							"userName": "u-user1"
						}
					],
					"note": "includes bindings for the groups the user belongs to: github_team://1234"
				},
				"uiContext": [
					{"cluster": "local", "kind": "ProjectRoleTemplateBinding", "name": "prtb-3", "namespace": "local-p-xyz", "type": "projectroletemplatebinding"},
					{"cluster": "local", "kind": "ProjectRoleTemplateBinding", "name": "prtb-group", "namespace": "local-p-xyz", "type": "projectroletemplatebinding"},
					{"cluster": "local", "kind": "ProjectRoleTemplateBinding", "name": "prtb-1", "namespace": "p-abc", "type": "projectroletemplatebinding"}
				]
			}`,
		},
		"filter by project and user": {
			params:  listPRTBParams{Cluster: "local", ProjectID: "p-abc", User: "user1"},
			objects: []runtime.Object{projectABC, prtb1, prtb2, prtb3},
			expectedResult: `{
				"llm": {
					"resources": [
						{
							"apiVersion": "management.cattle.io/v3",
							"kind": "ProjectRoleTemplateBinding",
							"metadata": {"name": "prtb-1", "namespace": "p-abc"},
							"projectName": "local:p-abc",
							"roleTemplateName": "project-owner",
							"userName": "u-user1"
						}
					],
					"note": "includes bindings for the groups the user belongs to: github_team://1234"
				},
				"uiContext": [
					{"cluster": "local", "kind": "ProjectRoleTemplateBinding", "name": "prtb-1", "namespace": "p-abc", "type": "projectroletemplatebinding"}
				]
			}`,
		},
		"user and group cannot both be specified": {
			params:        listPRTBParams{Cluster: "local", User: "u-user1", Group: "github_team://1234"},
			objects:       []runtime.Object{prtb1},
			expectedError: "only one of user or group can be specified",
		},
		"no PRTBs found": {
			params:         listPRTBParams{Cluster: "local"},
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

			result, _, err := tools.listProjectRoleTemplateBindings(
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

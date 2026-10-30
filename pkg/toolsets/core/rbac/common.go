package rbac

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/rancher/rancher-ai-mcp/internal/middleware"
	"github.com/rancher/rancher-ai-mcp/pkg/client"
	"github.com/rancher/rancher-ai-mcp/pkg/response"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var errUserAndGroup = errors.New("only one of user or group can be specified, bindings for the user's groups are included when a user is specified")

func filterResourcesByField(resources []*unstructured.Unstructured, field, value string) []*unstructured.Unstructured {
	var filteredResources []*unstructured.Unstructured
	for _, resource := range resources {
		resourceValue, found, err := unstructured.NestedString(resource.Object, field)
		if err == nil && found && resourceValue == value {
			filteredResources = append(filteredResources, resource)
		}
	}
	return filteredResources
}

// removeServerManagedFields removes fields maintained by the server, so responses and plans
// only show what is relevant to the resource.
func removeServerManagedFields(obj *unstructured.Unstructured) {
	unstructured.RemoveNestedField(obj.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(obj.Object, "status")
}

type deletedResources struct {
	Resources []map[string]any `json:"resources"`
	Note      string           `json:"note"`
}

// createDeletedResourcesResponse creates an MCP response describing deleted resources.
// It has no UI context because the resources no longer exist and cannot be linked to.
func createDeletedResourcesResponse(objs []*unstructured.Unstructured, note string) (string, error) {
	resources := make([]map[string]any, 0, len(objs))
	for _, obj := range objs {
		resources = append(resources, obj.Object)
	}

	return response.CreateMcpResponseAny(deletedResources{Resources: resources, Note: note})
}

// userSubjects resolves a username or user ID to the user ID, and the principal names of
// the groups the user belongs to, so that bindings for the user's groups can be found.
func (t *Tools) userSubjects(ctx context.Context, nameOrID string) (string, []string, error) {
	userID, err := t.resolveUserID(ctx, nameOrID)
	if err != nil {
		return "", nil, err
	}

	// Rancher records the groups a user belongs to in the UserAttribute with the same name as the user.
	// It is only created once the user logs in, so a missing UserAttribute means the user has no groups.
	userAttribute, err := t.client.GetResource(ctx, client.GetParams{
		Cluster: "local",
		Kind:    "userattribute",
		Name:    userID,
		Token:   middleware.Token(ctx),
	})
	if apierrors.IsNotFound(err) {
		return userID, nil, nil
	}
	if err != nil {
		return "", nil, fmt.Errorf("failed to get groups for user %q: %w", userID, err)
	}

	var groups []string
	groupPrincipals, _, _ := unstructured.NestedMap(userAttribute.Object, "groupPrincipals")
	for _, provider := range groupPrincipals {
		providerMap, ok := provider.(map[string]any)
		if !ok {
			continue
		}
		items, _, _ := unstructured.NestedSlice(providerMap, "items")
		for _, item := range items {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if name, found, err := unstructured.NestedString(itemMap, "metadata", "name"); err == nil && found && name != "" {
				groups = append(groups, name)
			}
		}
	}
	slices.Sort(groups)

	return userID, slices.Compact(groups), nil
}

// filterBindingsForUser returns the bindings for the user, or for any of the groups the user belongs to.
func filterBindingsForUser(bindings []*unstructured.Unstructured, userID string, groups []string) []*unstructured.Unstructured {
	var filtered []*unstructured.Unstructured
	for _, binding := range bindings {
		if userName, found, err := unstructured.NestedString(binding.Object, "userName"); err == nil && found && userName == userID {
			filtered = append(filtered, binding)
			continue
		}
		if group, found, err := unstructured.NestedString(binding.Object, "groupPrincipalName"); err == nil && found && slices.Contains(groups, group) {
			filtered = append(filtered, binding)
		}
	}
	return filtered
}

// filterBindingsForGroup returns the bindings for the group. Bindings normally record the group in
// groupPrincipalName, but cluster and project role template bindings can also use groupName.
func filterBindingsForGroup(bindings []*unstructured.Unstructured, group string) []*unstructured.Unstructured {
	var filtered []*unstructured.Unstructured
	for _, binding := range bindings {
		for _, field := range []string{"groupPrincipalName", "groupName"} {
			if value, found, err := unstructured.NestedString(binding.Object, field); err == nil && found && value == group {
				filtered = append(filtered, binding)
				break
			}
		}
	}
	return filtered
}

// userGroupsNote describes the groups whose bindings were included for a user, if any.
func userGroupsNote(groups []string) []string {
	if len(groups) == 0 {
		return nil
	}
	return []string{"includes bindings for the groups the user belongs to: " + strings.Join(groups, ", ")}
}

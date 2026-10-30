package rbac

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rancher/rancher-ai-mcp/internal/middleware"
	"github.com/rancher/rancher-ai-mcp/pkg/client"
	"github.com/rancher/rancher-ai-mcp/pkg/response"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var zapGetUser = zap.String("tool", "getUser")

type getUserParams struct {
	Username string `json:"username" jsonschema:"the username of the user to retrieve"`
}

func (t *Tools) getUser(ctx context.Context, toolReq *mcp.CallToolRequest, params getUserParams) (*mcp.CallToolResult, any, error) {
	zap.L().Debug("getUser called")

	user, err := t.findUser(ctx, params.Username)
	if err != nil {
		zap.L().Error("failed to get users", zapGetUser, zap.Error(err))
		return nil, nil, err
	}

	var matchedUser []*unstructured.Unstructured
	if user != nil {
		matchedUser = append(matchedUser, user)
	}

	mcpResponse, err := response.CreateMcpResponse(matchedUser, "local")
	if err != nil {
		zap.L().Error("failed to create mcp response", zapGetUser, zap.Error(err))
		return nil, nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: mcpResponse}},
	}, nil, nil
}

// findUser returns the first user whose username or metadata.name matches nameOrID, or nil if there is no match.
// It's possible that a user could have a username that matches another user's metadata.name, but this is unlikely since metadata.name is generated.
// We will return the first match we find in a best effort approach.
func (t *Tools) findUser(ctx context.Context, nameOrID string) (*unstructured.Unstructured, error) {
	users, err := t.client.GetResources(ctx, client.ListParams{
		Cluster: "local",
		Kind:    "user",
		Token:   middleware.Token(ctx),
	})
	if err != nil {
		return nil, err
	}

	for _, u := range users {
		if userName, found, err := unstructured.NestedString(u.Object, "username"); err == nil && found && userName == nameOrID {
			return u, nil
		}
		if metadataName, found, err := unstructured.NestedString(u.Object, "metadata", "name"); err == nil && found && metadataName == nameOrID {
			return u, nil
		}
	}

	return nil, nil
}

// resolveUserID returns the user ID (metadata.name) of the user whose username or ID matches nameOrID.
func (t *Tools) resolveUserID(ctx context.Context, nameOrID string) (string, error) {
	user, err := t.findUser(ctx, nameOrID)
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", fmt.Errorf("user %q not found, specify a username or user ID", nameOrID)
	}

	return user.GetName(), nil
}

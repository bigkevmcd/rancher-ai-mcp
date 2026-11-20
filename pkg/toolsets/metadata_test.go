package toolsets

import (
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/rancher/rancher-ai-mcp/pkg/toolsets/metadata"
)

// listAllTools lists the tools of a server with all tools registered.
func listAllTools(t *testing.T, readOnly bool) map[string]*mcp.Tool {
	t.Helper()

	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	// A nil client is safe here: tool handlers are never invoked, only listed.
	AddAllTools(nil, server, readOnly)

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })
	clientSession, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil).Connect(t.Context(), clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clientSession.Close() })

	tools := map[string]*mcp.Tool{}

	for tool, err := range clientSession.Tools(t.Context(), nil) {
		require.NoError(t, err)

		tools[tool.Name] = tool
	}

	return tools
}

func requiresHumanValidation(tool *mcp.Tool) bool {
	required, ok := tool.Meta[metadata.RequiresHumanValidationAnn].(bool)
	return ok && required
}

// TestWriteToolsRequireHumanValidation makes sure that every tool that changes resources declares it in its _meta
// with metadata.RequiresHumanValidationAnn, and can be previewed with a "<name>Plan" tool, so that clients can ask
// the user to confirm the change. Tools only registered in the full set are the ones that write.
func TestWriteToolsRequireHumanValidation(t *testing.T) {
	all := listAllTools(t, false)
	readOnly := listAllTools(t, true)

	for name, tool := range all {
		_, isReadOnly := readOnly[name]
		base := strings.TrimSuffix(name, "Plan")
		_, isPlan := all[base]
		isPlan = isPlan && base != name

		switch {
		case isReadOnly:
			require.False(t, requiresHumanValidation(tool), "read-only tool %s must not require human validation", name)
		case isPlan:
			require.False(t, requiresHumanValidation(tool), "plan tool %s only previews a change, it must not require human validation", name)
		default:
			require.True(t, requiresHumanValidation(tool), "tool %s changes resources: set %s in its Meta", name, metadata.RequiresHumanValidationAnn)
			require.Contains(t, all, name+"Plan", "tool %s changes resources: add a %sPlan tool to preview the change", name, name)
		}
	}
}

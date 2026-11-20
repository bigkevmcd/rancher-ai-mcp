// Package metadata defines the keys of the MCP tool _meta that clients rely on.
package metadata

const (
	// ToolsSetAnn is the key of the tool _meta that holds the toolset of a tool. Tools that are in several
	// toolsets can use a comma separated list.
	ToolsSetAnn = "toolset"

	// RequiresHumanValidationAnn can be applied to tool so that clients can ask
	// the user to confirm the change before calling the tool. The change is
	// previewed with the tool that has the same name followed by "Plan", for
	// example createProjectPlan for createProject.
	RequiresHumanValidationAnn = "requiresHumanValidation"
)

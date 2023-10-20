// The provider package implements a Jenkins provider for Terraform
//
// node_helpers.go implements utility methods for converting between the underlying
// gojenkins library into the appropriate Terraform types.
package provider

import (
	"context"
	"errors"
	"strings"

	"github.com/aidanleuck/gojenkins"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func (n *NodeResourceModel) createJNLPLauncher(ctx context.Context, lc *LauncherConfiguration, d diag.Diagnostics) (*gojenkins.JNLPLauncher, diag.Diagnostics) {
	// Given the case we got a JNLP launch type we first initialize the default launcher.
	defaultLauncher := gojenkins.DefaultJNLPLauncher()

	// If no options were provided then we will use the default launcher configuration.
	if lc.JNLPOptions.IsNull() {
		// Return default launcher.
		return defaultLauncher, nil
	}

	// Convert the launcher configuration from a Terraform object.
	var jnlpLauncherOptions JNLPOptions
	diags := n.UnmarshalLauncherConfiguration(ctx, lc)
	if diags.HasError() {
		return nil, diags
	}

	// Get the data provided by Terraform and set the launcher configuration based off that data.
	if !jnlpLauncherOptions.FailIfWorkDirMissing.IsNull() {
		defaultLauncher.WorkDirSettings.FailIfWorkDirIsMissing = jnlpLauncherOptions.FailIfWorkDirMissing.ValueBool()
	}
	if !jnlpLauncherOptions.RemoteDir.IsNull() {
		defaultLauncher.WorkDirSettings.InternalDir = jnlpLauncherOptions.RemoteDir.ValueString()
	}
	if !jnlpLauncherOptions.WebSocket.IsNull() {
		defaultLauncher.WebSocket = jnlpLauncherOptions.WebSocket.ValueBool()
	}
	if !jnlpLauncherOptions.WorkDirDisabled.IsNull() && !jnlpLauncherOptions.WorkDirDisabled.IsUnknown() {
		defaultLauncher.WorkDirSettings.Disabled = jnlpLauncherOptions.WorkDirDisabled.ValueBool()
	}

	return defaultLauncher, d
}

func (n *NodeResourceModel) createSSHLauncher(ctx context.Context, lc *LauncherConfiguration, d diag.Diagnostics) (*gojenkins.SSHLauncher, diag.Diagnostics) {
	// Given the case the user wants to create a SSH node, initialize a default launcher with Jenkins defaults
	defaultSSHLauncher := gojenkins.DefaultSSHLauncher()

	// If the user did not provide any additional options return the default launcher.
	if lc.SSHOptions.IsNull() {
		return defaultSSHLauncher, nil
	}

	// Get the user ssh_options data.
	var sshOptionConfig SSHOptions
	diags := lc.SSHOptions.As(ctx, &sshOptionConfig, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	// Host is a required field when using ssh_options
	defaultSSHLauncher.Host = sshOptionConfig.Host.ValueString()

	// Grab and parse all the optional types and append launcher information to the launcher.
	if !sshOptionConfig.CredentialsID.IsNull() {
		defaultSSHLauncher.CredentialsId = sshOptionConfig.CredentialsID.ValueString()
	}
	if !sshOptionConfig.JavaPath.IsNull() {
		defaultSSHLauncher.JavaPath = sshOptionConfig.JavaPath.ValueString()
	}
	if !sshOptionConfig.JvmOptions.IsNull() {
		defaultSSHLauncher.JvmOptions = sshOptionConfig.JvmOptions.ValueString()
	}
	if !sshOptionConfig.LaunchTimeoutSeconds.IsNull() {
		defaultSSHLauncher.LaunchTimeoutSeconds = int(sshOptionConfig.LaunchTimeoutSeconds.ValueInt64())
	}
	if !sshOptionConfig.MaxNumRetries.IsNull() {
		defaultSSHLauncher.MaxNumRetries = int(sshOptionConfig.MaxNumRetries.ValueInt64())
	}
	if !sshOptionConfig.Port.IsNull() {
		defaultSSHLauncher.Port = int(sshOptionConfig.Port.ValueInt64())
	}
	if !sshOptionConfig.PrefixStartSlaveCmd.IsNull() {
		defaultSSHLauncher.PrefixStartSlaveCmd = sshOptionConfig.PrefixStartSlaveCmd.ValueString()
	}
	if !sshOptionConfig.RetryWaitTime.IsNull() {
		defaultSSHLauncher.RetryWaitTime = int(sshOptionConfig.RetryWaitTime.ValueInt64())
	}
	if !sshOptionConfig.SuffixStartSlaveCmd.IsNull() {
		defaultSSHLauncher.SuffixStartSlaveCmd = sshOptionConfig.SuffixStartSlaveCmd.ValueString()
	}

	return defaultSSHLauncher, nil
}

// CreateLauncher creates the appropriate launcher based off user input from Terraform.
func (n *NodeResourceModel) CreateLauncher(ctx context.Context, d diag.Diagnostics) (gojenkins.Launcher, diag.Diagnostics) {
	var launcherConfig LauncherConfiguration
	diags := n.LauncherConfiguration.As(ctx, &launcherConfig, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	// Based off whether user input was JNLP or SSH return the appropriate launcher data.
	switch strings.ToLower(launcherConfig.Type.ValueString()) {
	case JNLPLauncherType:
		jnlpLauncher, diags := n.createJNLPLauncher(ctx, &launcherConfig, d)
		if diags.HasError() {
			return nil, diags
		}
		return jnlpLauncher, d

	// User wants an SSH node.
	case SSHLauncherType:
		sshLauncher, diags := n.createSSHLauncher(ctx, &launcherConfig, d)
		if diags.HasError() {
			return nil, diags
		}

		return sshLauncher, d
	default:
		d.AddError("unsupported launcher type", "launcher type must be ssh or jnlp")
		return nil, d
	}
}

func (n *NodeResourceModel) MergeJenkinsConfiguration(ctx context.Context, d diag.Diagnostics) {

}

// convertLabelsStr takes the Terraform labels list and converts it to a string separated string.
func (n *NodeResourceModel) ConvertLabelsStr(ctx context.Context) (string, diag.Diagnostics) {
	// By default labels will just be an empty slice
	defaultLabel := []string{}
	var labelElements []string

	// If labels isn't set, set labels to a empty string
	if n.Labels.IsUnknown() || n.Labels.IsNull() {
		labelElements = defaultLabel
	}

	// Make a new array with enough data to hold the labels
	labelElements = make([]string, 0, len(n.Labels.Elements()))

	// Convert from the Terraform type to go slice.
	diags := n.Labels.ElementsAs(ctx, &labelElements, false)

	// Error will get reported back to user abort mission.
	if diags.HasError() {
		return "", diags
	}

	// Join the string and return
	labelStr := strings.Join(labelElements, " ")
	return labelStr, diags
}

// Converts labels from a string to a Terraform list type.
func (n *NodeResourceModel) ConvertLabelsList(ctx context.Context, labels string) diag.Diagnostics {
	// Convert labels from space separated string to a slice.
	labelsString := strings.Split(labels, " ")
	labelsList, diag := types.ListValueFrom(ctx, types.StringType, labelsString)
	if diag.HasError() {
		return diag
	}
	n.Labels = labelsList
	return diag
}

// GetJNLPSecretTF converts the JNLP secret to its underlying Terraform type.
// If the agent is not a JNLP agent we set the value to null.
func GetJNLPSecretTF(ctx context.Context, n *gojenkins.Node, d diag.Diagnostics) (types.String, diag.Diagnostics) {
	// Check if the agent is a JNLP agent. If it is not set the value to null.
	ok, err := n.IsJnlpAgent(ctx)
	if !ok || err != nil {
		return types.StringNull(), d
	}

	// Attempt to grab the secret
	secret, err := n.GetJNLPSecret(ctx)
	if err != nil {
		d.AddError("failed to get jnlp secret", err.Error())
		return types.StringNull(), d
	}

	// Return the secret as a Terraform string value.
	return types.StringValue(secret), d
}

// Returns the JNLP attribute map
func getJNLPAttributes() map[string]attr.Type {
	return map[string]attr.Type{
		"use_web_socket":          types.BoolType,
		"workdir_disabled":        types.BoolType,
		"fail_if_workdir_missing": types.BoolType,
		"remoting_dir":            types.StringType,
	}
}

// Returns the launcher attribute map.
func getLauncherAttributes() map[string]attr.Type {
	return map[string]attr.Type{
		"launch_type":  types.StringType,
		"ssh_options":  types.ObjectType{AttrTypes: getSSHAttributes()},
		"jnlp_options": types.ObjectType{AttrTypes: getJNLPAttributes()},
	}
}

// Returns the SSH attribute map.
func getSSHAttributes() map[string]attr.Type {
	return map[string]attr.Type{
		"host":                   types.StringType,
		"port":                   types.Int64Type,
		"credentials_id":         types.StringType,
		"launch_timeout_seconds": types.Int64Type,
		"max_num_retries":        types.Int64Type,
		"retry_wait_time":        types.Int64Type,
		"jvm_options":            types.StringType,
		"java_path":              types.StringType,
		"prefix_start_slave_cmd": types.StringType,
		"suffix_start_slave_cmd": types.StringType,
	}
}

func convertJenkinsLauncherToTerraform(ctx context.Context, s *gojenkins.Slave) (string, error) {
	switch s.Launcher.Launcher.(type) {
	case *gojenkins.JNLPLauncher:
		return "jnlp", nil
	case *gojenkins.SSHLauncher:
		return "ssh", nil
	default:
		return "", errors.New("unsupported launcher type")
	}
}

// GetLauncher parses the launcher from the rest api response and converts it into a Terraform type.
func GetLauncher(ctx context.Context, s *gojenkins.Slave, d diag.Diagnostics) (*types.Object, diag.Diagnostics) {
	// Do reflection to determine what type of Jenkins slave we have.
	launcherConfiguration := &LauncherConfiguration{}
	switch l := s.Launcher.Launcher.(type) {
	case *gojenkins.JNLPLauncher:
		// Build up the go struct with the types from the Jenkins response.
		tfJnlpConfig := &JNLPOptions{
			WebSocket:            types.BoolValue(l.WebSocket),
			WorkDirDisabled:      types.BoolValue(l.WorkDirSettings.Disabled),
			FailIfWorkDirMissing: types.BoolValue(l.WorkDirSettings.FailIfWorkDirIsMissing),
			RemoteDir:            types.StringValue(l.WorkDirSettings.InternalDir),
		}

		// Convert the go struct to the Terraform object.
		jnlpTfObject, diag := types.ObjectValueFrom(ctx, getJNLPAttributes(), tfJnlpConfig)
		if diag.HasError() {
			return nil, diag
		}

		// Set the launcher configuration with JNLP options field, ssh options will be null.
		launcherConfiguration.JNLPOptions = jnlpTfObject
		launcherConfiguration.SSHOptions = types.ObjectNull(getSSHAttributes())
	case *gojenkins.SSHLauncher:
		// Create the SSH options Terraform struct based off rest api call.
		tfSSHConfig := &SSHOptions{
			Host:                 types.StringValue(l.Host),
			Port:                 types.Int64Value(int64(l.Port)),
			CredentialsID:        types.StringValue(l.CredentialsId),
			LaunchTimeoutSeconds: types.Int64Value(int64(l.LaunchTimeoutSeconds)),
			MaxNumRetries:        types.Int64Value(int64(l.MaxNumRetries)),
			RetryWaitTime:        types.Int64Value(int64(l.RetryWaitTime)),
			JvmOptions:           types.StringValue(l.JvmOptions),
			JavaPath:             types.StringValue(l.JavaPath),
			PrefixStartSlaveCmd:  types.StringValue(l.PrefixStartSlaveCmd),
			SuffixStartSlaveCmd:  types.StringValue(l.SuffixStartSlaveCmd),
		}

		// Convert the Terraform go struct to a Terraform object.
		sshTfObject, diag := types.ObjectValueFrom(ctx, getSSHAttributes(), tfSSHConfig)
		if diag.HasError() {
			return nil, diag
		}

		// Set the JNLP options to null, and set the ssh terraform object.
		launcherConfiguration.JNLPOptions = types.ObjectNull(getJNLPAttributes())
		launcherConfiguration.SSHOptions = sshTfObject
	default:
		d.AddError("unsupported launcher type", "must be ssh or jnlp")
		return nil, d
	}

	// Set the type of the launcher
	tfType, err := convertJenkinsLauncherToTerraform(ctx, s)
	if err != nil {
		d.AddError("failed to convert launcher to terraform type", err.Error())
	}
	launcherConfiguration.Type = types.StringValue(tfType)

	// Create the full launcher configuration object.
	tfLauncherObject, diag := types.ObjectValueFrom(ctx, getLauncherAttributes(), launcherConfiguration)
	if diag.HasError() {
		return nil, diag
	}
	return &tfLauncherObject, d
}

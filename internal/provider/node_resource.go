// The provider package implements a Jenkins provider for Terraform
//
// node_resource.go implements the node resource in Terraform using the gojenkins library.
package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aidanleuck/gojenkins"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &NodeResource{}
var _ resource.ResourceWithImportState = &NodeResource{}

const (
	JNLPLauncherType string = "jnlp"
	SSHLauncherType  string = "ssh"
)

// NewNodeResource returns a Node Resource struct.
func NewNodeResource() resource.Resource {
	return &NodeResource{}
}

// ExampleResource defines the resource implementation.
type NodeResource struct {
	client *gojenkins.Jenkins
}

// JNLP Options maps the Terraform schema configuration to its go type.
type JNLPOptions struct {
	WebSocket            types.Bool   `tfsdk:"use_web_socket"`
	WorkDirDisabled      types.Bool   `tfsdk:"workdir_disabled"`
	FailIfWorkDirMissing types.Bool   `tfsdk:"fail_if_workdir_missing"`
	RemoteDir            types.String `tfsdk:"remoting_dir"`
}

func (j *JNLPOptions) UnmarshalJNLPOptions(ctx context.Context, l *LauncherConfiguration) diag.Diagnostics {
	return l.JNLPOptions.As(ctx, j, basetypes.ObjectAsOptions{})
}

func (j *JNLPOptions) MarshalJNLPOptions(ctx context.Context, l *LauncherConfiguration) (types.Object, diag.Diagnostics) {
	return types.ObjectValueFrom(ctx, getJNLPAttributes(), j)
}

// SSH Options maps the ssh_option terraform schema to its go types.
type SSHOptions struct {
	Host                 types.String `tfsdk:"host"`
	Port                 types.Int64  `tfsdk:"port"`
	CredentialsID        types.String `tfsdk:"credentials_id"`
	LaunchTimeoutSeconds types.Int64  `tfsdk:"launch_timeout_seconds"`
	MaxNumRetries        types.Int64  `tfsdk:"max_num_retries"`
	RetryWaitTime        types.Int64  `tfsdk:"retry_wait_time"`
	JvmOptions           types.String `tfsdk:"jvm_options"`
	JavaPath             types.String `tfsdk:"java_path"`
	PrefixStartSlaveCmd  types.String `tfsdk:"prefix_start_slave_cmd"`
	SuffixStartSlaveCmd  types.String `tfsdk:"suffix_start_slave_cmd"`
}

func (s *SSHOptions) UnmarshalSSHOptions(ctx context.Context, l *LauncherConfiguration) diag.Diagnostics {
	return l.SSHOptions.As(ctx, s, basetypes.ObjectAsOptions{})
}

func (s *SSHOptions) MarshalSSHOptions(ctx context.Context) (types.Object, diag.Diagnostics) {
	return types.ObjectValueFrom(ctx, getSSHAttributes(), s)
}

// Launcher configuration maps launcher_configuration to its go type.
type LauncherConfiguration struct {
	JNLPOptions types.Object `tfsdk:"jnlp_options"`
	Type        types.String `tfsdk:"launch_type"`
	SSHOptions  types.Object `tfsdk:"ssh_options"`
}

func (n *NodeResourceModel) UnmarshalLauncherConfiguration(ctx context.Context, l *LauncherConfiguration) diag.Diagnostics {
	return n.LauncherConfiguration.As(ctx, l, basetypes.ObjectAsOptions{})
}

func (l *LauncherConfiguration) MarshalLauncherConfiguration(ctx context.Context) (types.Object, diag.Diagnostics) {
	return types.ObjectValueFrom(ctx, getLauncherAttributes(), l)
}

// NodeResourceModel describes the resource data model.
type NodeResourceModel struct {
	Name                  types.String `tfsdk:"name"`
	NumExecutors          types.Int64  `tfsdk:"executors"`
	Description           types.String `tfsdk:"description"`
	RemoteFS              types.String `tfsdk:"remote_fs"`
	Labels                types.List   `tfsdk:"labels"`
	JNLPSecret            types.String `tfsdk:"jnlp_secret"`
	LauncherConfiguration types.Object `tfsdk:"launcher_configuration"`
}

// Metadata sets the name of the resource. Which is provider name + type + _node.
func (r *NodeResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_node"
}

// Schema defines the schema for the resource.
func (r *NodeResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Creates a Jenkins Node.",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the Jenkins node",
				Required:            true,
			},
			"executors": schema.Int64Attribute{
				MarkdownDescription: "Number of executors",
				Optional:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Node description",
				Optional:            true,
			},
			"remote_fs": schema.StringAttribute{
				MarkdownDescription: "Where agent will store info/workspace",
				Required:            true,
			},
			"labels": schema.ListAttribute{
				MarkdownDescription: "labels to apply to the Jenkins node.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"jnlp_secret": schema.StringAttribute{
				MarkdownDescription: "computed value that is set for jnlp nodes.",
				Computed:            true,
			},
			"launcher_configuration": schema.SingleNestedAttribute{
				MarkdownDescription: "Defines launcher options for the node.",
				Attributes: map[string]schema.Attribute{
					"launch_type": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Type of launcher to connect to. Should be ssh or jnlp",
					},
					"ssh_options": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"host": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "IP address or DNS name of the machine to connect to.",
							},
							"port": schema.Int64Attribute{
								Optional:            true,
								MarkdownDescription: "Port to connect via SSH.",
							},
							"credentials_id": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "Credential to use to connect to Jenkins",
							},
							"launch_timeout_seconds": schema.Int64Attribute{
								Optional:            true,
								MarkdownDescription: "How long to wait before timing out a connection attempt.",
							},
							"max_num_retries": schema.Int64Attribute{
								Optional:            true,
								MarkdownDescription: "Max number of retries before the SSH agent stops attempting connection.",
							},
							"retry_wait_time": schema.Int64Attribute{
								Optional:            true,
								MarkdownDescription: "How long to wait between attempted SSH connections.",
							},
							"jvm_options": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "Additional flags to pass to Java",
							},
							"java_path": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "Path to Java bin to connect with.",
							},
							"prefix_start_slave_cmd": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "Command that runs before the agent connects to Jenkins.",
							},
							"suffix_start_slave_cmd": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "Command that runs after the agent connects to Jenkins.",
							},
						},
						MarkdownDescription: "Options for configuring an SSH agent.",
					},
					"jnlp_options": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"use_web_socket": schema.BoolAttribute{
								Optional:            true,
								MarkdownDescription: "Uses a websocket to connect.",
							},
							"workdir_disabled": schema.BoolAttribute{
								Optional:            true,
								MarkdownDescription: "Doesn't save data to the workspace.",
							},
							"fail_if_workdir_missing": schema.BoolAttribute{
								Optional:            true,
								MarkdownDescription: "Fails to build if work directory is missing.",
							},
							"remoting_dir": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "Directory where Jenkins stores agent data.",
							},
						},
						Optional:            true,
						MarkdownDescription: "Options for configuring a JNLP agent.",
					},
				},
				Required: true,
			},
		},
	}
}

func (r *NodeResourceModel) UpdateComputedResources(ctx context.Context, n *gojenkins.Node) error {
	tfSecret, err := GetJNLPSecretTF(ctx, n)
	if err != nil {
		return err
	}

	r.JNLPSecret = tfSecret
	return nil
}

// ValidateConfig adds custom validation to the schema. We use this to throw an error if both jnlp_options and ssh_options is set at one time.
func (r *NodeResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data NodeResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read the launcher configuration into a struct.
	var launcherConfig LauncherConfiguration
	diags := data.LauncherConfiguration.As(ctx, &launcherConfig, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return
	}

	// Return error messages if ssh and jnlp_options are set or they are using ssh_options with a jnlp launcher.
	lowerLauncher := strings.ToLower(launcherConfig.Type.ValueString())
	if lowerLauncher == "ssh" && !launcherConfig.JNLPOptions.IsNull() {
		resp.Diagnostics.AddError("can't use jnlp_options with ssh launcher type", "ssh launch type should be used with ssh_options block")
		return
	}
	if lowerLauncher == "jnlp" && !launcherConfig.SSHOptions.IsNull() {
		resp.Diagnostics.AddError("can't use ssh_options with jnlp launcher type", "jnlp launch type should be used with jnlp_options block")
		return
	}
}

// Configure cofigures the resource with required data from the provider.
func (r *NodeResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*gojenkins.Jenkins)

	// If we weren't able to get a Jenkins instance from the provider data something is wrong. Give up
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *gojenkins.Jenkins, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

// Create creates a new Jenkins node when one doesn't exist in state.
func (r *NodeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data NodeResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	jenkinsLauncher, err := data.CreateLauncher(ctx)
	if err != nil {
		resp.Diagnostics.AddError("failed to create launcher", err.Error())
		return
	}

	labels, err := data.ConvertLabelsStr(ctx)
	if err != nil {
		resp.Diagnostics.AddError("failed to convert labels to string", err.Error())
		return
	}

	// Create the node.
	node, err := r.client.CreateNode(ctx,
		data.Name.ValueString(),
		int(data.NumExecutors.ValueInt64()),
		data.Description.ValueString(),
		data.RemoteFS.ValueString(),
		labels,
		jenkinsLauncher,
	)
	if err != nil {
		resp.Diagnostics.AddError("failed to create node", err.Error())
		return
	}

	if err = data.UpdateComputedResources(ctx, node); err != nil {
		resp.Diagnostics.AddError("failed updating computed resources", err.Error())
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NodeResourceModel) MergeConfiguration(ctx context.Context, n *gojenkins.Node) error {
	nodeConfiguration, err := n.GetLauncherConfig(ctx)
	if err != nil {
		return err
	}

	if err = r.ConvertLabelsList(ctx, nodeConfiguration.Label); err != nil {
		return err
	}

	if !r.Description.IsNull() {
		r.Description = types.StringValue(nodeConfiguration.Description)
	}

	if !r.NumExecutors.IsNull() {
		r.NumExecutors = types.Int64Value(int64(nodeConfiguration.NumExecutors))
	}

	r.Name = types.StringValue(nodeConfiguration.Name)
	r.RemoteFS = types.StringValue(nodeConfiguration.RemoteFS)

	if err = r.mergeLauncherConfiguration(ctx, nodeConfiguration.Launcher.Launcher); err != nil {
		return err
	}

	return nil
}

func (r *NodeResourceModel) mergeLauncherConfiguration(ctx context.Context, currentConfig gojenkins.Launcher) error {
	var tfLauncherConfiguration LauncherConfiguration

	// If there are no user defined launcher configurations, abort
	if r.LauncherConfiguration.IsNull() {
		return nil
	}

	diags := r.LauncherConfiguration.As(ctx, &tfLauncherConfiguration, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return errors.New("failed converting launcher configuration")
	}
	switch l := currentConfig.(type) {
	case *gojenkins.JNLPLauncher:
		tfLauncherConfiguration.SSHOptions = types.ObjectNull(getSSHAttributes())
		tfLauncherConfiguration.Type = types.StringValue(JNLPLauncherType)
		if tfLauncherConfiguration.JNLPOptions.IsNull() || tfLauncherConfiguration.JNLPOptions.IsUnknown() {
			tfLauncherConfiguration.JNLPOptions = types.ObjectNull(getJNLPAttributes())
			break
		}
		var currentJnlpConfig JNLPOptions
		diags = tfLauncherConfiguration.JNLPOptions.As(ctx, &currentJnlpConfig, basetypes.ObjectAsOptions{})
		if diags.HasError() {
			return errors.New("failed converting jnlp options")
		}

		if !currentJnlpConfig.FailIfWorkDirMissing.IsNull() {
			currentJnlpConfig.FailIfWorkDirMissing = types.BoolValue(l.WorkDirSettings.FailIfWorkDirIsMissing)
		}
		if !currentJnlpConfig.RemoteDir.IsNull() {
			currentJnlpConfig.RemoteDir = types.StringValue(l.WorkDirSettings.InternalDir)
		}
		if !currentJnlpConfig.WebSocket.IsNull() {
			currentJnlpConfig.WebSocket = types.BoolValue(l.WebSocket)
		}
		if !currentJnlpConfig.WorkDirDisabled.IsNull() {
			currentJnlpConfig.WorkDirDisabled = types.BoolValue(l.WorkDirSettings.Disabled)
		}

		jnlpObject, diags := types.ObjectValueFrom(ctx, getJNLPAttributes(), &currentJnlpConfig)
		if diags.HasError() {
			return errors.New("failed converting from jnlp struct to object")
		}

		tfLauncherConfiguration.JNLPOptions = jnlpObject
	case *gojenkins.SSHLauncher:
		tfLauncherConfiguration.JNLPOptions = types.ObjectNull(getJNLPAttributes())
		tfLauncherConfiguration.Type = types.StringValue(SSHLauncherType)
		var currentSSHLauncher SSHOptions
		// If we don't know about an ssh launcher we don't need to continue
		if tfLauncherConfiguration.SSHOptions.IsNull() || tfLauncherConfiguration.SSHOptions.IsUnknown() {
			tfLauncherConfiguration.SSHOptions = types.ObjectNull(getSSHAttributes())
			break
		}

		diags := tfLauncherConfiguration.SSHOptions.As(ctx, &currentSSHLauncher, basetypes.ObjectAsOptions{})
		if diags.HasError() {
			return errors.New("failed converting ssh launcher to terraform object")
		}

		if !currentSSHLauncher.CredentialsID.IsNull() {
			currentSSHLauncher.CredentialsID = types.StringValue(l.CredentialsId)
		}
		if !currentSSHLauncher.Host.IsNull() {
			currentSSHLauncher.Host = types.StringValue(l.Host)
		}
		if !currentSSHLauncher.JavaPath.IsNull() {
			currentSSHLauncher.JavaPath = types.StringValue(l.JavaPath)
		}
		if !currentSSHLauncher.JvmOptions.IsNull() {
			currentSSHLauncher.JvmOptions = types.StringValue(l.JvmOptions)
		}
		if !currentSSHLauncher.LaunchTimeoutSeconds.IsNull() {
			currentSSHLauncher.LaunchTimeoutSeconds = types.Int64Value(int64(l.LaunchTimeoutSeconds))
		}
		if !currentSSHLauncher.MaxNumRetries.IsNull() {
			currentSSHLauncher.MaxNumRetries = types.Int64Value(int64(l.MaxNumRetries))
		}
		if !currentSSHLauncher.Port.IsNull() {
			currentSSHLauncher.Port = types.Int64Value(int64(l.Port))
		}
		if !currentSSHLauncher.PrefixStartSlaveCmd.IsNull() {
			currentSSHLauncher.PrefixStartSlaveCmd = types.StringValue(l.PrefixStartSlaveCmd)
		}
		if !currentSSHLauncher.RetryWaitTime.IsNull() {
			currentSSHLauncher.RetryWaitTime = types.Int64Value(int64(l.RetryWaitTime))
		}
		if !currentSSHLauncher.SuffixStartSlaveCmd.IsNull() {
			currentSSHLauncher.SuffixStartSlaveCmd = types.StringValue(l.SuffixStartSlaveCmd)
		}

		sshObject, diags := types.ObjectValueFrom(ctx, getSSHAttributes(), &currentSSHLauncher)
		if diags.HasError() {
			return errors.New("failed converting from ssh struct to object")
		}

		tfLauncherConfiguration.SSHOptions = sshObject

	default:
		return errors.New("unsupported launcher type. must be ssh or jnlp launcher")

	}
	launcherObject, diags := types.ObjectValueFrom(ctx, getLauncherAttributes(), &tfLauncherConfiguration)
	if diags.HasError() {
		return errors.New("failed converting launcher to terraform object")
	}

	r.LauncherConfiguration = launcherObject

	return nil
}

// Read reads the state from Jenkins and attempts to synchronize it.
func (r *NodeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data NodeResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the node
	node, err := r.client.GetNode(ctx, data.Name.ValueString())
	if err != nil {
		errMessage := fmt.Sprintf("failed to find jenkins node %s", data.Name.ValueString())
		resp.Diagnostics.AddError(errMessage, err.Error())
		return
	}

	if err := data.MergeConfiguration(ctx, node); err != nil {
		resp.Diagnostics.AddError("failed merging configuration", err.Error())
		return
	}

	if err := data.UpdateComputedResources(ctx, node); err != nil {
		resp.Diagnostics.AddError("failed updating computed resources", err.Error())
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NodeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data NodeResourceModel
	var prevData NodeResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the old state
	resp.Diagnostics.Append(req.State.Get(ctx, &prevData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the state of the current node
	// We read from the previous state in case the name has changed.
	node, err := r.client.GetNode(ctx, prevData.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to find node", err.Error())
		return
	}

	// Generate the new launcher configuration
	jenkinsLauncher, err := data.CreateLauncher(ctx)
	if err != nil {
		resp.Diagnostics.AddError("failed creating launcher when updating launcher", err.Error())
		return
	}

	// Get the labels from user configuration
	labels, err := data.ConvertLabelsStr(ctx)
	if err != nil {
		resp.Diagnostics.AddError("failed converting label to string", err.Error())
		return
	}

	// Send a request to Jenkins to update the node
	newNode, err := node.UpdateNode(ctx,
		data.Name.ValueString(),
		int(data.NumExecutors.ValueInt64()),
		data.Description.ValueString(),
		data.RemoteFS.ValueString(),
		labels,
		jenkinsLauncher,
	)
	if err != nil {
		resp.Diagnostics.AddError("failed to update node", err.Error())
		return
	}

	// Update the computer resources
	if err = data.UpdateComputedResources(ctx, newNode); err != nil {
		resp.Diagnostics.AddError("failed updating computed resources", err.Error())
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NodeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data NodeResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	node, err := r.client.GetNode(ctx, data.Name.ValueString())
	if err != nil {
		errMessage := fmt.Sprintf("failed to find jenkins node %s", data.Name.ValueString())
		resp.Diagnostics.AddError(errMessage, err.Error())
		return
	}

	ok, err := node.Delete(ctx)
	if !ok || err != nil {
		errMessage := fmt.Sprintf("failed to delete jenkins node %s", data.Name.ValueString())
		resp.Diagnostics.AddError(errMessage, err.Error())
		return
	}
}

func (r *NodeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

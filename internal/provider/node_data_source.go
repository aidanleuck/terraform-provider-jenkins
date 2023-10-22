// The provider package implements a Jenkins provider for Terraform
//
// node_data_source.go implements a node_data_source for Jenkins. This allows
// the user to gather information about an exisitng node in Jenkins and use it
// in Terraform.
package provider

import (
	"context"
	"fmt"

	"github.com/aidanleuck/gojenkins"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Force go to throw a compile error if node data source
// does not implement the data source interface.
var _ datasource.DataSource = &NodeDataSource{}

// Holds required data for the node data source.
type NodeDataSource struct {
	client *gojenkins.Jenkins
}

// Node data source model maps the Terraform schema to go types.
type NodeDataSourceModel struct {
	Name                  types.String `tfsdk:"name"`
	NumExecutors          types.Int64  `tfsdk:"executors"`
	Description           types.String `tfsdk:"description"`
	RemoteFS              types.String `tfsdk:"remote_fs"`
	Labels                types.List   `tfsdk:"labels"`
	JNLPSecret            types.String `tfsdk:"jnlp_secret"`
	LauncherConfiguration types.Object `tfsdk:"launcher_configuration"`
}

// Metadata exports the name of the data source with is provider + type + _node.
func (d *NodeDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_node"
}

// NewNodeDataSource initializes a blank data source.
func NewNodeDataSource() datasource.DataSource {
	return &NodeDataSource{}
}

// Configure sets up the data source with the Jenkins client.
func (d *NodeDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	// Set the Jenkins client.
	d.client = client
}

// Schema maps the schema returned by the data source.
func (d *NodeDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
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
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Node description",
				Computed:            true,
			},
			"remote_fs": schema.StringAttribute{
				MarkdownDescription: "Where agent will store info",
				Computed:            true,
			},
			"labels": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
			},
			"jnlp_secret": schema.StringAttribute{
				Computed: true,
			},
			"launcher_configuration": schema.ObjectAttribute{
				AttributeTypes: getLauncherAttributes(),
				Computed:       true,
			},
		},
	}
}

// Read reads the Node data from Jenkins and converts it into a Terraform type that can be consumed by the user.
func (d *NodeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data NodeDataSourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Search for the node
	node, err := d.client.GetNode(ctx, data.Name.ValueString())
	if err != nil {
		errMessage := fmt.Sprintf("failed to find jenkins node %s", data.Name.ValueString())
		resp.Diagnostics.AddError(errMessage, err.Error())
		return
	}

	// Get node configuration
	slaveConfig, err := node.GetLauncherConfig(ctx)
	if err != nil {
		errMessage := fmt.Sprintf("failed to retrieve jenkins launcher config %s", data.Name.ValueString())
		resp.Diagnostics.AddError(errMessage, err.Error())
		return
	}

	// Get the JNLP secret if applicable.
	retrievedSecret, err := GetJNLPSecretTF(ctx, node)
	if err != nil {
		resp.Diagnostics.AddError("failed retrieving terraform secret", err.Error())
		return
	}

	// Set Terraform model data.
	data.JNLPSecret = retrievedSecret
	data.NumExecutors = types.Int64Value(int64(slaveConfig.NumExecutors))
	data.Description = types.StringValue(slaveConfig.Description)

	labelsList, err := convertLabelList(ctx, slaveConfig.Label)
	if err != nil {
		resp.Diagnostics.AddError("failed converting labels to terraform list", err.Error())
		return
	}

	// Grab some other information from the base launch configuration
	data.Labels = labelsList
	data.Name = types.StringValue(node.GetName())
	data.RemoteFS = types.StringValue(slaveConfig.RemoteFS)

	// Parse custom launcher information specific to the launcher plugin (SSH or JNLP configuration)
	launcher, err := GetLauncher(ctx, slaveConfig)
	if err != nil {
		resp.Diagnostics.AddError("failed retrieving launcher from Jenkins", err.Error())
		return
	}

	data.LauncherConfiguration = *launcher

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

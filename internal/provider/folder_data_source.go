// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// The provider package implements a Jenkins provider for Terraform
//
// folder_data_source.go implements a folder data source for Jenkins. This allows
// the user to gather information about an existing folder in Jenkins.
package provider

import (
	"context"
	"fmt"

	"github.com/aidanleuck/gojenkins"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &FolderDataSource{}

// FolderDataSource holds required data for the folder data source.
type FolderDataSource struct {
	client *gojenkins.Jenkins
}

// FolderDataSourceModel maps the Terraform schema to go types.
type FolderDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Folder      types.String `tfsdk:"folder"`
	Description types.String `tfsdk:"description"`
	URL         types.String `tfsdk:"url"`
}

// NewFolderDataSource initializes a blank data source.
func NewFolderDataSource() datasource.DataSource {
	return &FolderDataSource{}
}

// Metadata exports the name of the data source which is provider + type + _folder.
func (d *FolderDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_folder"
}

// Configure sets up the data source with the Jenkins client.
func (d *FolderDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*gojenkins.Jenkins)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *gojenkins.Jenkins, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

// Schema maps the schema returned by the data source.
func (d *FolderDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads information about an existing Jenkins folder. Requires the `cloudbees-folder` plugin.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Full path of the folder, for example `parent/child`.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the folder.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(folderNameRegex, "must not be empty or contain `/`"),
				},
			},
			"folder": schema.StringAttribute{
				MarkdownDescription: "Path of the parent folder. Leave unset to look the folder up at the Jenkins root.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(folderPathRegex, "must not be empty and must not start or end with `/`"),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the folder.",
				Computed:            true,
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "URL of the folder in Jenkins.",
				Computed:            true,
			},
		},
	}
}

// Read reads the folder from Jenkins.
func (d *FolderDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data FolderDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := buildFolderID(data.Folder.ValueString(), data.Name.ValueString())
	folder, found, err := getFolder(ctx, d.client, data.Folder.ValueString(), data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to retrieve jenkins folder %s", id), err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("folder not found", fmt.Sprintf("jenkins folder %s does not exist", id))
		return
	}

	data.ID = types.StringValue(id)
	data.Description = types.StringValue(folder.Raw.Description)
	data.URL = types.StringValue(folder.Raw.URL)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

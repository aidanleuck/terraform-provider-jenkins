// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// The provider package implements a Jenkins provider for Terraform
//
// folder_data_source.go implements a folder data source for Jenkins. This allows
// the user to gather information about an existing folder in Jenkins and use it
// in Terraform.
package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/aidanleuck/gojenkins"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Force go to throw a compile error if folder data source
// does not implement the data source interface.
var _ datasource.DataSource = &FolderDataSource{}

// Holds required data for the folder data source.
type FolderDataSource struct {
	client *gojenkins.Jenkins
}

// Folder data source model maps the Terraform schema to go types.
type FolderDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Parent      types.String `tfsdk:"parent"`
	Description types.String `tfsdk:"description"`
}

// NewFolderDataSource initializes a blank data source.
func NewFolderDataSource() datasource.DataSource {
	return &FolderDataSource{}
}

// Metadata exports the name of the data source with is provider + type + _folder.
func (d *FolderDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_folder"
}

// Configure sets up the data source with the Jenkins client.
func (d *FolderDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*gojenkins.Jenkins)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *gojenkins.Jenkins, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.client = client
}

// Schema maps the schema returned by the data source.
func (d *FolderDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Gets information about an existing Jenkins Folder. Requires the cloudbees-folder plugin.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Full path of the folder, for example `team-a/services`.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the Jenkins folder",
				Required:            true,
			},
			"parent": schema.StringAttribute{
				MarkdownDescription: "Path of the parent folder, for example `team-a/services`. Leave unset to look up a folder at the Jenkins root.",
				Optional:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Folder description",
				Computed:            true,
			},
		},
	}
}

// Read reads the folder data from Jenkins and converts it into a Terraform type that can be consumed by the user.
func (d *FolderDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data FolderDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()
	parent := data.Parent.ValueString()

	folder, err := getFolder(ctx, d.client, parent, name)
	if err != nil {
		if errors.Is(err, gojenkins.ErrNotFound) {
			resp.Diagnostics.AddError("folder not found", fmt.Sprintf("folder %s was not found in Jenkins.", folderID(parent, name)))
			return
		}

		resp.Diagnostics.AddError(fmt.Sprintf("error occurred retrieving data for folder %s", folderID(parent, name)), err.Error())
		return
	}

	data.ID = types.StringValue(folderID(parent, name))
	data.Description = types.StringValue(folder.Raw.Description)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

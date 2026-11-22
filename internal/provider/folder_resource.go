// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// The provider package implements a Jenkins provider for Terraform
//
// folder_resource.go implements the folder resource in Terraform using the gojenkins library.
package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/aidanleuck/gojenkins"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &FolderResource{}
var _ resource.ResourceWithImportState = &FolderResource{}

// NewFolderResource returns a Folder Resource struct.
func NewFolderResource() resource.Resource {
	return &FolderResource{}
}

// FolderResource defines the resource implementation.
type FolderResource struct {
	client *gojenkins.Jenkins
}

// FolderResourceModel describes the resource data model.
type FolderResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Parent      types.String `tfsdk:"parent"`
	Description types.String `tfsdk:"description"`
}

// Metadata sets the name of the resource. Which is provider name + type + _folder.
func (r *FolderResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_folder"
}

// Schema defines the schema for the resource.
func (r *FolderResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Creates a Jenkins Folder. Requires the cloudbees-folder plugin.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Full path of the folder, for example `team-a/services`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the folder. Changing this forces a new folder to be created.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					stringvalidator.RegexMatches(folderNameRegex, "must not contain a '/'"),
				},
			},
			"parent": schema.StringAttribute{
				MarkdownDescription: "Path of the parent folder, for example `team-a/services`. Leave unset to create the folder at the Jenkins root. Changing this forces a new folder to be created.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(folderParentRegex, "must be a path of folder names separated by '/', without leading or trailing '/'"),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the folder.",
				Optional:            true,
			},
		},
	}
}

// Configure sets up the resource with the Jenkins client.
func (r *FolderResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = client
}

// Create creates the folder in Jenkins.
func (r *FolderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data FolderResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()
	parent := data.Parent.ValueString()

	if err := ensureParentExists(ctx, r.client, parent); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("parent"), "parent folder not found", err.Error())
		return
	}

	folder, err := r.client.CreateFolder(ctx, name, splitFolderPath(parent)...)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to create folder %s", folderID(parent, name)), err.Error())
		return
	}

	if description := data.Description.ValueString(); description != "" {
		if err := setFolderDescription(ctx, r.client, folder, description); err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("failed to set description of folder %s", folderID(parent, name)), err.Error())
			return
		}
	}

	data.ID = types.StringValue(folderID(parent, name))

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read reads the state from Jenkins and attempts to synchronize it.
func (r *FolderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data FolderResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	parent, name := splitFolderID(data.ID.ValueString())

	folder, err := getFolder(ctx, r.client, parent, name)
	if err != nil {
		// Folder has been deleted
		if errors.Is(err, gojenkins.ErrNotFound) {
			resp.Diagnostics.AddWarning("folder not found", fmt.Sprintf("folder %s was not found in Jenkins.", data.ID.ValueString()))
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(fmt.Sprintf("error occurred retrieving data for folder %s", data.ID.ValueString()), err.Error())
		return
	}

	data.Name = types.StringValue(name)
	if parent == "" {
		data.Parent = types.StringNull()
	} else {
		data.Parent = types.StringValue(parent)
	}

	// Treat an empty description the same as an unset one.
	if folder.Raw.Description == "" {
		data.Description = types.StringNull()
	} else {
		data.Description = types.StringValue(folder.Raw.Description)
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update updates the description of the folder in place. Name and parent changes force replacement.
func (r *FolderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data FolderResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	parent := data.Parent.ValueString()
	name := data.Name.ValueString()

	folder, err := getFolder(ctx, r.client, parent, name)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to find folder %s", folderID(parent, name)), err.Error())
		return
	}

	if err := setFolderDescription(ctx, r.client, folder, data.Description.ValueString()); err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to update description of folder %s", folderID(parent, name)), err.Error())
		return
	}

	data.ID = types.StringValue(folderID(parent, name))

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete deletes the folder, and everything it contains, from Jenkins.
func (r *FolderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data FolderResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	parent, name := splitFolderID(data.ID.ValueString())

	folder, err := getFolder(ctx, r.client, parent, name)
	if err != nil {
		// Nothing left to delete.
		if errors.Is(err, gojenkins.ErrNotFound) {
			return
		}

		resp.Diagnostics.AddError(fmt.Sprintf("failed to find jenkins folder %s", data.ID.ValueString()), err.Error())
		return
	}

	if err := folder.Delete(ctx); err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to delete jenkins folder %s", data.ID.ValueString()), err.Error())
		return
	}
}

// ImportState imports a folder by its full path, for example team-a/services.
func (r *FolderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

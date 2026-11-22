// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// The provider package implements a Jenkins provider for Terraform
//
// folder_resource.go implements the folder resource in Terraform using the gojenkins library.
// Folders require the Jenkins cloudbees-folder plugin.
package provider

import (
	"context"
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

// FolderResourceModel maps the Terraform schema to go types.
type FolderResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Folder      types.String `tfsdk:"folder"`
	Description types.String `tfsdk:"description"`
	URL         types.String `tfsdk:"url"`
}

// Metadata exports the name of the resource which is provider + type + _folder.
func (r *FolderResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_folder"
}

// Schema maps the folder schema.
func (r *FolderResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a Jenkins folder. Requires the `cloudbees-folder` plugin.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Full path of the folder, for example `parent/child`.",
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
					stringvalidator.RegexMatches(folderNameRegex, "must not be empty or contain `/`"),
				},
			},
			"folder": schema.StringAttribute{
				MarkdownDescription: "Path of the parent folder, for example `parent` or `grandparent/parent`. " +
					"Leave unset to create the folder at the Jenkins root. The parent folder must already exist. " +
					"Changing this forces a new folder to be created.",
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(folderPathRegex, "must not be empty and must not start or end with `/`"),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the folder.",
				Optional:            true,
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "URL of the folder in Jenkins.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
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

// Create creates a folder in Jenkins.
func (r *FolderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data FolderResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := data.Name.ValueString()
	parent := data.Folder.ValueString()

	folder, err := r.client.CreateFolder(ctx, name, splitFolderPath(parent)...)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to create jenkins folder %s", buildFolderID(parent, name)), err.Error())
		return
	}

	id := buildFolderID(parent, name)

	// gojenkins discards the poll error when creating, so poll explicitly to
	// make sure the folder data (notably the URL) is populated.
	if _, err := folder.Poll(ctx); err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to retrieve created jenkins folder %s", id), err.Error())
		return
	}

	data.ID = types.StringValue(id)
	data.URL = types.StringValue(folder.Raw.URL)

	if !data.Description.IsNull() && data.Description.ValueString() != "" {
		if err := setFolderDescription(ctx, r.client, folder, data.Description.ValueString()); err != nil {
			resp.Diagnostics.AddError(fmt.Sprintf("failed to set description of jenkins folder %s", id), err.Error())
			// The folder exists, so save it to state. Because the diagnostics
			// contain an error, Terraform marks the resource as tainted and
			// replaces it on the next apply, which retries the description.
			data.Description = types.StringNull()
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read reads the folder from Jenkins and removes it from state if it no longer exists.
func (r *FolderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data FolderResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := buildFolderID(data.Folder.ValueString(), data.Name.ValueString())
	folder, found, err := getFolder(ctx, r.client, data.Folder.ValueString(), data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("error occurred retrieving data for folder %s", id), err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddWarning("folder not found", fmt.Sprintf("folder %s was not found in Jenkins.", id))
		resp.State.RemoveResource(ctx)
		return
	}

	data.ID = types.StringValue(id)
	data.URL = types.StringValue(folder.Raw.URL)
	// Remote changes to the description are intentionally surfaced as drift, so the
	// remote value always wins here. Keep a null description only when neither
	// side has one to avoid a perpetual diff.
	if folder.Raw.Description != "" || !data.Description.IsNull() {
		data.Description = types.StringValue(folder.Raw.Description)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update updates the folder description. All other attributes force replacement.
func (r *FolderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data FolderResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := buildFolderID(data.Folder.ValueString(), data.Name.ValueString())
	folder, found, err := getFolder(ctx, r.client, data.Folder.ValueString(), data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to find jenkins folder %s", id), err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("folder not found", fmt.Sprintf("folder %s was not found in Jenkins.", id))
		return
	}

	if err := setFolderDescription(ctx, r.client, folder, data.Description.ValueString()); err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to update jenkins folder %s", id), err.Error())
		return
	}

	if _, err := folder.Poll(ctx); err != nil {
		resp.Diagnostics.AddError("failed to refresh jenkins folder", err.Error())
		return
	}

	data.ID = types.StringValue(id)
	data.URL = types.StringValue(folder.Raw.URL)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete deletes the folder, including everything it contains.
func (r *FolderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data FolderResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := buildFolderID(data.Folder.ValueString(), data.Name.ValueString())
	folder, found, err := getFolder(ctx, r.client, data.Folder.ValueString(), data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to find jenkins folder %s", id), err.Error())
		return
	}
	if !found {
		// Already gone, nothing to do.
		return
	}

	if err := folder.Delete(ctx); err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("failed to delete jenkins folder %s", id), err.Error())
	}
}

// ImportState imports a folder by its full path, for example `parent/child`.
func (r *FolderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parent, name, err := parseFolderID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("invalid import id", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
	if parent != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("folder"), parent)...)
	}
}

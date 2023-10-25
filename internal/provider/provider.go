// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/aidanleuck/gojenkins"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure JenkinsProvider satisfies various provider interfaces.
var _ provider.Provider = &JenkinsProvider{}

const (
	JENKINS_USERNAME_ENV_KEY string = "JENKINS_PROVIDER_USERNAME"
	JENKINS_PASSWORD_ENV_KEY string = "JENKINS_PROVIDER_PASSWORD"
)

// JenkinsProvider defines the provider implementation.
type JenkinsProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// ScaffoldingProviderModel describes the provider data model.
type JenkinsProviderModel struct {
	Url      types.String `tfsdk:"url"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
	CACert   types.String `tfsdk:"ca_cert"`
}

func (p *JenkinsProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "jenkins"
	resp.Version = p.version
}

func (p *JenkinsProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"url": schema.StringAttribute{
				MarkdownDescription: "URL to the Jenkins Server to configure",
				Required:            true,
			},
			"username": schema.StringAttribute{
				MarkdownDescription: "Username to authenticate to the Jenkins Server",
				Optional:            true,
				Sensitive:           true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Password to authenticate to the Jenkins Server",
				Optional:            true,
				Sensitive:           true,
			},
			"ca_cert": schema.StringAttribute{
				MarkdownDescription: "CA certificate to use if you are authenticating with a server that is using a self signed cert.",
				Optional:            true,
				Sensitive:           true,
			},
		},
	}
}

func (p *JenkinsProviderModel) getUsername() (string, error) {
	if !p.Username.IsNull() {
		return p.Username.ValueString(), nil
	}

	username, ok := os.LookupEnv(JENKINS_USERNAME_ENV_KEY)
	if !ok {
		return "", fmt.Errorf("username is required, set environment variable %s or the username field under provider", JENKINS_USERNAME_ENV_KEY)
	}

	return username, nil
}

func (p *JenkinsProviderModel) getPassword() (string, error) {
	if !p.Password.IsNull() {
		return p.Password.ValueString(), nil
	}

	password, ok := os.LookupEnv(JENKINS_PASSWORD_ENV_KEY)
	if !ok {
		return "", fmt.Errorf("password is required, set environment variable %s or the username field under provider", JENKINS_PASSWORD_ENV_KEY)
	}

	return password, nil
}

func (p *JenkinsProvider) ValidateConfig(ctx context.Context, req provider.ValidateConfigRequest, resp *provider.ValidateConfigResponse) {
	var data JenkinsProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	// return if there are errors.
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate CACert is valid.
	if !data.CACert.IsNull() {
		_, err := os.Stat(data.CACert.ValueString())
		if err != nil {
			errMessage := fmt.Sprintf("%s%s%s", "path", data.CACert.ValueString(), "doesn't exist")
			resp.Diagnostics.AddAttributeError(path.Root("ca_cert"), errMessage, errMessage)
			return
		}
	}

	// Make sure user has valid username and password
	if _, err := data.getPassword(); err != nil {
		resp.Diagnostics.AddError("failed getting password", err.Error())
		return
	}

	if _, err := data.getUsername(); err != nil {
		resp.Diagnostics.AddError("failed getting username", err.Error())
		return
	}
}

func (p *JenkinsProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data JenkinsProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	// Return if there was an error parsing the provider schema
	// Plugin SDK handles errors
	if resp.Diagnostics.HasError() {
		return
	}

	// Make sure user has valid username and password
	password, err := data.getPassword()
	if err != nil {
		resp.Diagnostics.AddError("failed getting password", err.Error())
		return
	}

	username, err := data.getUsername()
	if err != nil {
		resp.Diagnostics.AddError("failed getting username", err.Error())
		return
	}

	jenkinsServer := gojenkins.CreateJenkins(nil, data.Url.ValueString(), username, password)

	// Configure ca cert to communicate with Jenkins
	if !data.CACert.IsNull() {
		cacert, err := ioutil.ReadFile(data.CACert.ValueString())
		if err != nil {
			errMessage := fmt.Sprintf("%s%s", "failed to read provided ca cert at path", data.CACert.ValueString())
			tflog.Error(ctx, errMessage)
			resp.Diagnostics.AddError(errMessage, err.Error())
			return
		}

		jenkinsServer.Requester.CACert = cacert
	}

	client, err := jenkinsServer.Init(ctx)

	// Return to prevent a panic.
	if err != nil {
		errMessage := "failed to communicate with jenkins"
		resp.Diagnostics.AddError(errMessage, err.Error())
		tflog.Error(ctx, errMessage)
		return
	}

	tflog.Info(ctx, "Connected to Jenkins!")

	// Use the jenkins client for resources and data sources!
	resp.ResourceData = client
	resp.DataSourceData = client
}

func (p *JenkinsProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewNodeResource,
	}
}

func (p *JenkinsProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewNodeDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &JenkinsProvider{
			version: version,
		}
	}
}

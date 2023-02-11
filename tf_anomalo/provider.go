package tf_anomalo

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/squareup/gonomalo"
)

// Ensure ScaffoldingProvider satisfies various provider interfaces.
var _ provider.Provider = &AnomaloProvider{}

type AnomaloProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &AnomaloProvider{version: version}
	}
}

func (p *AnomaloProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "tfanomalo"
	resp.Version = p.version
}

type ProviderModel struct {
	Host  types.String `tfsdk:"host"`
	Token types.String `tfsdk:"token"`
}

func (p *AnomaloProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				Optional: true,
			},
			"token": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
			},
		},
	}
}

func (p *AnomaloProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config ProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If practitioner provided a configuration value for any of the
	// attributes, it must be a known value.

	if config.Host.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("host"),
			"Unknown Anomalo API Host",
			"The provider cannot create the Anomalo API client as there is an unknown configuration value for the Anomalo API host. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the ANOMALO_HOST environment variable.",
		)
	}

	if config.Token.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("token"),
			"Unknown Anomalo API Token",
			"The provider cannot create the Anomalo API client as there is an unknown configuration value for the Anomalo API Token. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the ANOMALO_TOKEN environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Default values to environment variables, but override
	// with Terraform configuration value if set.

	host := os.Getenv("ANOMALO_HOST")
	token := os.Getenv("ANOMALO_TOKEN")

	if !config.Host.IsNull() {
		host = config.Host.ValueString()
	}

	if !config.Token.IsNull() {
		token = config.Token.ValueString()
	}

	// If any of the expected configurations are missing, return
	// errors with provider-specific guidance.

	if host == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("host"),
			"Missing Anomalo API Host",
			"The provider cannot create the Anomalo API client as there is a missing or empty value for the Anomalo API host. "+
				"Set the host value in the configuration or use the ANOMALO_HOST environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if token == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("token"),
			"Missing Anomalo API Token",
			"The provider cannot create the Anomalo API client as there is a missing or empty value for the Anomalo API token. "+
				"Set the token value in the configuration or use the ANOMALO_TOKEN environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	client := gonomalo.AnomaloClient{Token: token, Host: host}
	testCall, err := client.Ping()
	if err != nil || testCall.Ping != "pong" {
		resp.Diagnostics.AddError(
			"Unable to Create Anomalo API Client",
			"An unexpected error occurred when creating the Anomalo API client. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"Anomalo Client Error: "+err.Error(),
		)
		return
	}

	resp.DataSourceData = &client
	resp.ResourceData = &client
}

func (p *AnomaloProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewTableResource,
	}
}

// DataSources defines the data sources implemented in the provider.
func (p *AnomaloProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}

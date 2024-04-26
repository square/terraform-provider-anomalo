package anomalo

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/square/anomalo-go/anomalo"
)

// Ensure the implementation satisfies the expected interfaces.
var _ provider.Provider = &Provider{}

const (
	AnomaloHostEnvName     = "ANOMALO_INSTANCE_HOST"
	AnomaloTokenEnvVarName = "ANOMALO_API_SECRET_TOKEN"
)

// The Anomalo Provider
type Provider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &Provider{version: version}
	}
}

func (p Provider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "anomalo"
	resp.Version = p.version
}

type ProviderModel struct {
	Host         types.String `tfsdk:"host"`
	Token        types.String `tfsdk:"token"`
	Organization types.String `tfsdk:"organization"`
}

func (p Provider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Declarative configuration for Anomalo Tables & Checks.",
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				Optional:    true,
				Description: "Your anomalo API host. Ex `https://anomalo.mycompany.com`",
			},
			"token": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Your anomalo API token. Ex `j1ThisIsaFake%tokenMxJ`",
			},
			"organization": schema.StringAttribute{
				Optional: true,
				Description: "Optional - the name of the organization this API key should act within the scope of. " +
					"Ex. `Square`. The provider will not reset the organization after it finishes running. " +
					"Consumers of this provider can define multiple provider configurations and use Terraform's " +
					"Resource provider Meta-Argument.",
			},
		},
	}
}

func (p Provider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config ProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.Host.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("host"),
			"Unknown Anomalo API Host",
			fmt.Sprintf("The provider cannot create the Anomalo API client as there is an unknown configuration "+
				"value for the Anomalo API host. Either set the value statically in the configuration, or use the %s "+
				"environment variable.", AnomaloHostEnvName),
		)
	}

	if config.Token.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("token"),
			"Unknown Anomalo API Token",
			fmt.Sprintf("The provider cannot create the Anomalo API client as there is an unknown configuration "+
				"value for the Anomalo API Token. Either set the value statically in the configuration, or use the %s "+
				"environment variable.", AnomaloTokenEnvVarName),
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Use values in environment variables if the values aren't set in the configuration.

	hostEnv := os.Getenv(AnomaloHostEnvName)
	if !config.Host.IsNull() {
		hostEnv = config.Host.ValueString()
	}

	tokenEnv := os.Getenv(AnomaloTokenEnvVarName)
	if !config.Token.IsNull() {
		tokenEnv = config.Token.ValueString()
	}

	// Return errors if any of the expected configurations are missing

	if hostEnv == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("host"),
			"Missing Anomalo API host",
			fmt.Sprintf("The provider cannot create the Anomalo API client as there is a missing or empty "+
				"value for the Anomalo API host. Either set the value statically in the configuration, or use the %s "+
				"environment variable.", AnomaloHostEnvName),
		)
	}

	if tokenEnv == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("token"),
			"Unknown Anomalo API Token",
			fmt.Sprintf("The provider cannot create the Anomalo API client as there is a missing or empty "+
				"value for the ProviAnomaloder API Token. Either set the value statically in the configuration, or use the %s "+
				"environment variable.", AnomaloTokenEnvVarName),
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	client := anomalo.Client{Token: tokenEnv, Host: hostEnv, ClientProvider: func() *http.Client { return &http.Client{} }}
	testCall, err := client.Ping()
	if err != nil || testCall.Ping != "pong" {
		resp.Diagnostics.AddError(
			"Unable to Create a Working Anomalo Client",
			fmt.Sprintf("The provider was unable to make a request to the anomalo API with the provided host "+
				"& token. If the error is not clear, please contact the provider developers.\n Error: %s", err.Error()),
		)
		return
	}

	// If passed non-null non-empty organization, attempt to use it.
	if !(config.Organization.IsNull() || config.Organization.ValueString() == "") {
		org, err := client.GetOrganizationByName(config.Organization.ValueString())
		if err != nil || org == nil {
			resp.Diagnostics.AddError(
				"Unable to Fetch Organization",
				fmt.Sprintf("The provider was unable to fetch the provided organization '%s'. If the error is"+
					" not clear, please contact the provider developers.\n Error: %s",
					config.Organization.ValueString(), err.Error()),
			)
			return
		}

		changeOrgResp, err := client.ChangeOrganization(int64(org.ID))
		if err != nil || changeOrgResp.ID != org.ID {
			resp.Diagnostics.AddError(
				"Unable to Change Organization",
				fmt.Sprintf("The provider was unable to change to the provided organization '%s' with ID %d. "+
					"If the error is not clear, please contact the provider developers.\n Error: %s",
					config.Organization.ValueString(), org.ID, err.Error()),
			)
			return
		}
	}

	resp.DataSourceData = &client
	resp.ResourceData = &client
}

func (p Provider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		newTableResource,
		newCheckResource,
	}
}

func (p Provider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		newNotificationChannelDataSource,
	}
}

package anomalo

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/square/anomalo-go/anomalo"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &notificationChannelDataSource{}
	_ datasource.DataSourceWithConfigure = &notificationChannelDataSource{}
)

func newNotificationChannelDataSource() datasource.DataSource {
	return &notificationChannelDataSource{}
}

type notificationChannelDataSource struct {
	client *anomalo.Client
}

// Values expected in the state & configuration
type notificationChannelDataSourceModel struct {
	ID          types.Int64  `tfsdk:"id"`
	ChannelType types.String `tfsdk:"channel_type"`
	Name        types.String `tfsdk:"name"`
}

func (r *notificationChannelDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	r.client = req.ProviderData.(*anomalo.Client)
}

// Metadata returns the data source type name.
func (r *notificationChannelDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_notification_channel"
}

// Schema defines the schema for the data source.
func (r *notificationChannelDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "References a notification channel. A Notification Channel is a destination for alerts from failed checks. Ex. a Slack channel or email address.",
		Attributes: map[string]schema.Attribute{
			"channel_type": schema.StringAttribute{
				Required:    true,
				Description: "The type of notification channel. One of \"slack\" \"msteams\" \"pagerduty\" \"email\" \"email_all\"",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the notification channel. For example, \"@squareJake\". Because of a limitation in the Anomalo API, this may not work if the name is a substring of multiple channels (within the same `channel_type`)",
			},
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "A unique ID. Generated by Anomalo.",
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *notificationChannelDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state notificationChannelDataSourceModel
	diags := req.Config.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	channel, err := r.client.GetNotificationChannelWithDescriptionContaining(
		state.Name.ValueString(),
		state.ChannelType.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Fetch Notification Channel",
			fmt.Sprintf("Error %v encountered while searching for channel name %s type %s",
				err.Error(), state.Name, state.ChannelType),
		)
		return
	}
	if channel == nil {
		resp.Diagnostics.AddError(
			"Could not Find Notification Channel",
			fmt.Sprintf("Could not find notification channel with name %s and type %s",
				state.Name, state.ChannelType),
		)
		return
	}

	// Finish setting state with computed ID
	state.ID = types.Int64Value(int64(channel.ID))

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

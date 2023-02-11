package tf_anomalo

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/squareup/gonomalo"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &tableResource{}
	_ resource.ResourceWithConfigure = &tableResource{}
)

// NewTableResource is a helper function to simplify the provider implementation.
func NewTableResource() resource.Resource {
	return &tableResource{}
}

// tableResource is the resource implementation.
type tableResource struct {
	client *gonomalo.AnomaloClient
}

type tableResourceModel struct {
	TableName                 types.String `tfsdk:"table_name"`
	TableId                   types.Int64  `tfsdk:"table_id"`
	LastUpdated               types.String `tfsdk:"last_updated"`
	CheckCadenceType          types.String `tfsdk:"check_cadence_type"`
	CheckCadenceRunAtDuration types.String `tfsdk:"check_cadence_run_at_duration"`
	NotificationChannelID     types.Int64  `tfsdk:"notification_channel_id"`
}

func (r *tableResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	r.client = req.ProviderData.(*gonomalo.AnomaloClient)
}

// Metadata returns the resource type name.
func (r *tableResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_table"
}

// Schema defines the schema for the resource.
func (r *tableResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"table_id": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"last_updated": schema.StringAttribute{
				Computed: true,
			},
			"table_name": schema.StringAttribute{
				Required: true,
			},
			"check_cadence_type": schema.StringAttribute{
				Required: true,
			},
			"check_cadence_run_at_duration": schema.StringAttribute{
				Required: true,
			},
			"notification_channel_id": schema.Int64Attribute{
				Required: true,
			},
		},
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *tableResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan tableResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tableName := *removeQuotesIfPresent(plan.TableName.String())
	table, errStr, err := r.client.GetTableInformation(tableName)
	if err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error configuring table. Could not fetch table ID for table %d", len(plan.TableName.String())),
			"Unexpected error: "+" : "+err.Error(),
		)
		return
	} else if errStr != "" {
		resp.Diagnostics.AddError(
			"Error Reading HashiCups Order",
			fmt.Sprintf("Error configuring table. Could not fetch table ID for %s %s name %s errstr %s", r.client.Token, r.client.Host, plan.TableName.String(), errStr),
		)
		return
	}
	tableId := table.ID

	// Generate API request body from plan
	configureTableReq := gonomalo.ConfigureTableRequest{
		TableID:                   tableId,
		CheckCadenceType:          removeQuotesIfPresent(plan.CheckCadenceType.String()),
		CheckCadenceRunAtDuration: *removeQuotesIfPresent(plan.CheckCadenceRunAtDuration.String()),
		NotificationChannelID:     int(plan.NotificationChannelID.ValueInt64()),
	}

	// Create new table
	configureTableResponse, errStr, err := r.client.ConfigureTable(configureTableReq)
	//err = fmt.Errorf("asdf")
	if err != nil {
		resp.Diagnostics.AddError(
			"Error configuring table",
			fmt.Sprintf("Could not configure table, unexpected error: %v, %v, \n %v", err.Error(), configureTableReq, configureTableResponse),
		)
		return
	} else if errStr != "" {
		resp.Diagnostics.AddError(
			"Error Reading HashiCups Order",
			fmt.Sprintf("Error configuring table. Could not fetch table ID for %s %s name %s errstr %s", r.client.Token, r.client.Host, plan.TableName.String(), errStr),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	plan.TableId = types.Int64Value(int64(configureTableResponse.ID))
	plan.CheckCadenceType = types.StringValue(*configureTableReq.CheckCadenceType)
	plan.CheckCadenceRunAtDuration = types.StringValue(configureTableReq.CheckCadenceRunAtDuration)
	plan.NotificationChannelID = types.Int64Value(int64(configureTableReq.NotificationChannelID))
	plan.TableName = types.StringValue(tableName)
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *tableResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state tableResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get refreshed table value from HashiCups
	table, errStr, err := r.client.GetTableInformation(*removeQuotesIfPresent(state.TableName.String()))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading HashiCups Order",
			"Could not read HashiCups table name err: "+state.TableName.String()+" "+err.Error(),
		)
		return
	} else if errStr != "" {
		resp.Diagnostics.AddError(
			"Error Reading HashiCups Order",
			fmt.Sprintf("Could not read HashiCups table %s %s name %s errstr %s", r.client.Token, r.client.Host, state.TableName.String(), errStr),
		)
		return
	}

	// Overwrite local state with refreshed state
	state.CheckCadenceType = types.StringValue(table.Config.CheckCadenceType)
	state.TableName = types.StringValue(fmt.Sprintf("%s.%s", table.Warehouse.Name, table.FullName))
	state.TableId = types.Int64Value(int64(table.ID))
	state.CheckCadenceRunAtDuration = types.StringValue(table.Config.CheckCadenceRunAtDuration)
	state.NotificationChannelID = types.Int64Value(int64(table.Config.NotificationChannelID))

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *tableResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan tableResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tableName := *removeQuotesIfPresent(plan.TableName.String())
	table, errStr, err := r.client.GetTableInformation(tableName)
	if err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error configuring table. Could not fetch table ID for table %d", len(plan.TableName.String())),
			"Unexpected error: "+" : "+err.Error(),
		)
		return
	} else if errStr != "" {
		resp.Diagnostics.AddError(
			"Error Reading HashiCups Order",
			fmt.Sprintf("Error configuring table. Could not fetch table ID for %s %s name %s errstr %s", r.client.Token, r.client.Host, plan.TableName.String(), errStr),
		)
		return
	}
	tableId := table.ID

	// Generate API request body from plan
	configureTableReq := gonomalo.ConfigureTableRequest{
		TableID:                   tableId,
		CheckCadenceType:          removeQuotesIfPresent(plan.CheckCadenceType.String()),
		CheckCadenceRunAtDuration: *removeQuotesIfPresent(plan.CheckCadenceRunAtDuration.String()),
		NotificationChannelID:     int(plan.NotificationChannelID.ValueInt64()),
	}

	// Create new table
	configureTableResponse, errStr, err := r.client.ConfigureTable(configureTableReq)
	//err = fmt.Errorf("asdf")
	if err != nil {
		resp.Diagnostics.AddError(
			"Error configuring table",
			fmt.Sprintf("Could not configure table, unexpected error: %v, %v, \n %v", err.Error(), configureTableReq, configureTableResponse),
		)
		return
	} else if errStr != "" {
		resp.Diagnostics.AddError(
			"Error Reading HashiCups Order",
			fmt.Sprintf("Error configuring table. Could not fetch table ID for %s %s name %s errstr %s", r.client.Token, r.client.Host, plan.TableName.String(), errStr),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	plan.TableId = types.Int64Value(int64(configureTableResponse.ID))
	plan.CheckCadenceType = types.StringValue(*configureTableReq.CheckCadenceType)
	plan.CheckCadenceRunAtDuration = types.StringValue(configureTableReq.CheckCadenceRunAtDuration)
	plan.NotificationChannelID = types.Int64Value(int64(configureTableReq.NotificationChannelID))
	plan.TableName = types.StringValue(tableName)
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *tableResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state tableResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tableName := *removeQuotesIfPresent(state.TableName.String())
	table, errStr, err := r.client.GetTableInformation(tableName)
	if err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error configuring table. Could not fetch table ID for table %d", len(state.TableName.String())),
			"Unexpected error: "+" : "+err.Error(),
		)
		return
	} else if errStr != "" {
		resp.Diagnostics.AddError(
			"Error Reading HashiCups Order",
			fmt.Sprintf("Error configuring table. Could not fetch table ID for %s %s name %s errstr %s", r.client.Token, r.client.Host, state.TableName.String(), errStr),
		)
		return
	}
	tableId := table.ID

	// Delete existing order
	// Generate API request body from plan
	configureTableReq := gonomalo.ConfigureTableRequest{
		TableID:          tableId,
		CheckCadenceType: nil,
		//CheckCadenceRunAtDuration: "",
		//NotificationChannelID:     int(state.NotificationChannelID.ValueInt64()),
	}

	// Create new table
	configureTableResponse, errStr, err := r.client.ConfigureTable(configureTableReq)
	//err = fmt.Errorf("asdf")
	if err != nil {
		resp.Diagnostics.AddError(
			"Error configuring table",
			fmt.Sprintf("Could not configure table, unexpected error: %v, %v, \n %v", err.Error(), configureTableReq, configureTableResponse),
		)
		return
	} else if errStr != "" {
		resp.Diagnostics.AddError(
			"Error Reading HashiCups Order",
			fmt.Sprintf("Error configuring table. Could not fetch table ID for %s %s name %s errstr %s", r.client.Token, r.client.Host, state.TableName.String(), errStr),
		)
		return
	}
}

func removeQuotesIfPresent(str string) *string {
	if str[0] == '"' && str[len(str)-1] == '"' {
		val := str[1 : len(str)-1]
		return &val
	}
	return &str
}

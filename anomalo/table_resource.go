package anomalo

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	"github.com/hashicorp/terraform-plugin-framework/path"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/square/anomalo-go/anomalo"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &tableResource{}
	_ resource.ResourceWithConfigure   = &tableResource{}
	_ resource.ResourceWithImportState = &tableResource{}
)

func newTableResource() resource.Resource {
	return &tableResource{}
}

type tableResource struct {
	client *anomalo.Client
}

// Values expected in the state & configuration
type tableResourceModel struct {
	TableName                 types.String `tfsdk:"table_name"`
	TableID                   types.Int64  `tfsdk:"table_id"`
	CheckCadenceType          types.String `tfsdk:"check_cadence_type"`
	CheckCadenceRunAtDuration types.String `tfsdk:"check_cadence_run_at_duration"`
	NotificationChannelID     types.Int64  `tfsdk:"notification_channel_id"`
	Definition                types.String `tfsdk:"definition"`
	TimeColumnType            types.String `tfsdk:"time_column_type"`
	NotifyAfter               types.String `tfsdk:"notify_after"`
	FreshAfter                types.String `tfsdk:"fresh_after"`
	IntervalSkipExpr          types.String `tfsdk:"interval_skip_expr"`
	AlwaysAlertOnErrors       types.Bool   `tfsdk:"always_alert_on_errors"`
	TimeColumns               types.List   `tfsdk:"time_columns"`
}

func (r *tableResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	r.client = req.ProviderData.(*anomalo.Client)
}

// Metadata returns the resource type name.
func (r *tableResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_table"
}

// Schema defines the schema for the resource.
func (r *tableResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An Anomalo table's configuration. Maps closely to the Anomalo API for `configure_table`. See " +
			"your API documentation for more information on attributes.\n",
		Attributes: map[string]schema.Attribute{
			"table_id": schema.Int64Attribute{
				Computed: true,
				Optional: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
					int64planmodifier.RequiresReplaceIfConfigured(),
				},
				Description: "The ID of the table. Should not be set manually. Is Optional strictly to support more " +
					"forgiving imports.",
			},
			"table_name": schema.StringAttribute{
				Required: true,
				Description: "The fully qualified name of the table, including the warehouse. " +
					"Ex warehouse_name.schema_name.table_name",
			},
			"check_cadence_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					EmptyIfNull(),
				},
				Validators: []validator.String{
					// These are example validators from terraform-plugin-framework-validators
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^(daily|data_freshness_gated)$`),
						"Check cadence type must be null, 'daily', or 'data_freshness_gated'",
					),
				},
				Description: "How often checks should execute on this table. Exclude this attribute (or equivalently, " +
					"set to null) to turn off checks for the table. Acceptable values include null, " +
					"\"daily\", and \"daily_freshness_gated\"",
			},
			"check_cadence_run_at_duration": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					EmptyIfNull(),
				},
			},
			"notification_channel_id": schema.Int64Attribute{
				Required: true,
				Description: "Notification channel that this table's alerts should be sent to. " +
					"Can be used with the `NotificationChannel` data-source, " +
					"ex `anomalo_notification_channel.team_slack_channel.id`",
			},
			"definition": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					EmptyIfNull(),
				},
			},
			"time_column_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					EmptyIfNull(),
				},
			},
			"notify_after": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					EmptyIfNull(),
				},
			},
			"fresh_after": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					EmptyIfNull(),
				},
			},
			"interval_skip_expr": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					EmptyIfNull(),
				},
			},
			"always_alert_on_errors": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					BoolDefaultValue(types.BoolValue(false)),
				},
			},
			"time_columns": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					DefaultEmptyList(),
				},
			},
		},
	}
}

// Create creates the resource and sets the initial Terraform state. Note this method doesn't actually "create tables.
// Anomalo already has an ID for every table it knows about. This method "configures" a table.
func (r *tableResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan tableResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Confirm Anomalo knows about the table
	tableName := plan.TableName.ValueString()
	table, err := r.client.GetTableInformation(tableName)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Table",
			fmt.Sprintf("Tables must already exist in anomalo before being created. Could not fetch table ID "+
				"for table %s. Is it accessible by Anomalo? Unexpected error: %s", plan.TableName.String(), err.Error()),
		)
		return
	}
	tableID := table.ID

	// Populate API request body based on plan values
	configureTableReq := anomalo.ConfigureTableRequest{
		TableID:                   tableID,
		CheckCadenceType:          toPtr(plan.CheckCadenceType.ValueString()),
		CheckCadenceRunAtDuration: plan.CheckCadenceRunAtDuration.ValueString(),
		NotificationChannelID:     int(plan.NotificationChannelID.ValueInt64()),
		Definition:                plan.Definition.ValueString(),
		AlwaysAlertOnErrors:       plan.AlwaysAlertOnErrors.ValueBool(),
		TimeColumnType:            plan.TimeColumnType.ValueString(),
		NotifyAfter:               plan.NotifyAfter.ValueString(),
		FreshAfter:                plan.FreshAfter.ValueString(),
		IntervalSkipExpr:          plan.IntervalSkipExpr.ValueString(),
	}

	// Extract the list values from the plan and add to the request
	var target []string
	diags = plan.TimeColumns.ElementsAs(ctx, &target, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(target) > 0 {
		configureTableReq.TimeColumns = target
	}

	// Create new table configuration
	configureTableResponse, err := r.client.ConfigureTable(configureTableReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Table",
			fmt.Sprintf("Could not configure table %s, unexpected error: %s",
				plan.TableName.String(), err.Error()),
		)
		return
	}

	// Map response body back into the plan. Some values in the plan do not change based on the API response, and
	// do not need to be set again.
	plan.TableID = types.Int64Value(int64(configureTableResponse.ID))
	plan.NotificationChannelID = types.Int64Value(int64(configureTableReq.NotificationChannelID))
	plan.AlwaysAlertOnErrors = types.BoolValue(configureTableReq.AlwaysAlertOnErrors)

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

	table, err := r.client.GetTableInformation(state.TableName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Table",
			fmt.Sprintf("Could not read table configuration for table name %s, unexpected error: %v",
				state.TableName.ValueString(), err.Error()),
		)
		return
	}

	// Map non-collection attributes into the local state, and set the response state to plan values.
	state.TableName = types.StringValue(fmt.Sprintf("%s.%s", table.Warehouse.Name, table.FullName))
	state.TableID = types.Int64Value(int64(table.ID))
	state.NotificationChannelID = types.Int64Value(int64(table.Config.NotificationChannelID))
	state.AlwaysAlertOnErrors = types.BoolValue(table.Config.AlwaysAlertOnErrors)
	state.CheckCadenceRunAtDuration = types.StringValue(table.Config.CheckCadenceRunAtDuration)
	state.CheckCadenceType = types.StringValue(table.Config.CheckCadenceType)
	state.Definition = types.StringValue(table.Config.Definition)
	state.TimeColumnType = types.StringValue(table.Config.TimeColumnType)
	state.NotifyAfter = types.StringValue(table.Config.NotifyAfter)
	state.FreshAfter = types.StringValue(table.Config.FreshAfter)
	state.IntervalSkipExpr = types.StringValue(table.Config.IntervalSkipExpr)

	// Map the list values into the state
	var listVals []attr.Value
	for _, val := range table.Config.TimeColumns {
		// Note: consider using safer string conversion based on runtime type of val
		listVals = append(listVals, types.StringValue(fmt.Sprintf("%v", val)))
	}
	listTimeColumns, diags := types.ListValue(types.StringType, listVals)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.TimeColumns = listTimeColumns

	// Set response state to updated values
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *tableResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan tableResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Also retrieve values from state, so we can grab tableID
	var state tableResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tableID, diags := r.tableIdForState(state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate API request body from plan
	configureTableReq := anomalo.ConfigureTableRequest{
		TableID:                   tableID,
		CheckCadenceType:          toPtr(plan.CheckCadenceType.ValueString()),
		CheckCadenceRunAtDuration: plan.CheckCadenceRunAtDuration.ValueString(),
		NotificationChannelID:     int(plan.NotificationChannelID.ValueInt64()),
		Definition:                plan.Definition.ValueString(),
		AlwaysAlertOnErrors:       plan.AlwaysAlertOnErrors.ValueBool(),
		TimeColumnType:            plan.TimeColumnType.ValueString(),
		NotifyAfter:               plan.NotifyAfter.ValueString(),
		FreshAfter:                plan.FreshAfter.ValueString(),
		IntervalSkipExpr:          plan.IntervalSkipExpr.ValueString(),
	}

	// Add list-values from the plan to the request
	var target []string
	diags = plan.TimeColumns.ElementsAs(ctx, &target, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	configureTableReq.TimeColumns = target

	// Update the table
	configureTableResponse, err := r.client.ConfigureTable(configureTableReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Table",
			fmt.Sprintf("Could not Configure table %s, unexpected error: %s",
				plan.TableName.String(), err.Error()),
		)
		return
	}

	// Map response body back into the plan, and set the state to plan values. Some values in the plan do not
	// change based on the API response, so we leave them there.
	plan.TableID = types.Int64Value(int64(configureTableResponse.ID))
	plan.NotificationChannelID = types.Int64Value(int64(configureTableReq.NotificationChannelID))
	plan.AlwaysAlertOnErrors = types.BoolValue(configureTableReq.AlwaysAlertOnErrors)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *tableResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state tableResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tableID, diags := r.tableIdForState(state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTableReq := anomalo.ConfigureTableRequest{
		TableID:          tableID,
		CheckCadenceType: nil,
	}

	_, err := r.client.ConfigureTable(deleteTableReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Table",
			fmt.Sprintf("Could not Configure table %s, unexpected error: %s", state.TableName.String(), err.Error()),
		)
		return
	}
}

func (r *tableResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("table_name"), req, resp)
}

func (r *tableResource) tableIdForState(state tableResourceModel) (int, diag.Diagnostics) {
	if state.TableID.ValueInt64() > 0 {
		return int(state.TableID.ValueInt64()), nil
	} else {
		// This is unexpected, but table ID is not present in the state. Fetch it based on table name
		tableName := state.TableName.ValueString()
		table, err := r.client.GetTableInformation(tableName)
		if err != nil {
			diagErr := diag.NewErrorDiagnostic(
				"Error Deleting Table",
				fmt.Sprintf("Error fetching table ID. Could not fetch for table %s, unexpected error: %s",
					state.TableName.String(), err.Error()),
			)
			return 0, []diag.Diagnostic{diagErr}
		}
		return table.ID, nil
	}
}

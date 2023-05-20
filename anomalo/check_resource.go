package anomalo

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
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
	_ resource.Resource                = &checkResource{}
	_ resource.ResourceWithConfigure   = &checkResource{}
	_ resource.ResourceWithImportState = &checkResource{}
)

func newCheckResource() resource.Resource {
	return &checkResource{}
}

type checkResource struct {
	client *anomalo.Client
}

// Values expected in the state & configuration
type checkResourceModel struct {
	TableID       types.Int64  `tfsdk:"table_id"`
	CheckType     types.String `tfsdk:"check_type"`
	CheckStaticID types.Int64  `tfsdk:"check_static_id"`
	Params        types.Map    `tfsdk:"params"`
}

func (r *checkResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	r.client = req.ProviderData.(*anomalo.Client)
}

// Metadata returns the resource type name.
func (r *checkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_check"
}

// Schema defines the schema for the resource.
func (r *checkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An Anomalo check. Closely maps to the check object in the Anomalo API. Updating system checks (checks with negative IDs) is not supported by the Anomalo API and thus is not supported by this resource.",
		Attributes: map[string]schema.Attribute{
			"check_static_id": schema.Int64Attribute{
				Computed: true,
				Optional: true,
				PlanModifiers: []planmodifier.Int64{
					// In an ideal world (with a fully featured anomalo API) we could use
					// int64planmodifier.UseStateForUnknown. However we are unable to update checks in place & must
					// delete + recreate them.
					int64planmodifier.RequiresReplaceIfConfigured(),
				},
				Description: "The check ID, persists through updates. Implementation Detail: The Anomalo API implements check updates as a deletion of the old check + creation of a new one. When using this provider, you can ignore that detail by using `static_check_id`. This makes the resource behave like a typical HTTP resource.",
			},
			"table_id": schema.Int64Attribute{
				Computed: true,
				Optional: true,
				PlanModifiers: []planmodifier.Int64{
					// In an ideal world (with a fully featured anomalo API) we could use
					// int64planmodifier.UseStateForUnknown. However we are unable to update checks in place & must
					// delete + recreate them.
					int64planmodifier.RequiresReplaceIfConfigured(),
				},
				Description: "The ID of the table that this check belongs to. This can be specified by referencing " +
					"the resource object, ex `anomalo_table.<resource_name>.table_id`. It should not be changed after " +
					"creation.",
			},
			"check_type": schema.StringAttribute{
				Required: true,
				Description: "The type of check. Valid values are available in the Anomalo API documentation for " +
					"`create_check`.",
			},
			"params": schema.MapAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "A map of parameters for the provided check type. Valid values are available in the " +
					"Anomalo API documentation for `create_check`. Acceptable values differ by check type.",
			},
		},
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *checkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	//Retrieve values from plan
	var plan checkResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !plan.CheckStaticID.IsNull() && !plan.CheckStaticID.IsUnknown() {
		resp.Diagnostics.AddError(
			"Error Creating Check",
			fmt.Sprintf("Specifying a check_static_id during creation is not supported. If you are creating a "+
				"new check, set check_static_id to null. If you are trying to update an existing check, first import "+
				"it with `terraform import <terraform-resource-identifier> %d,%d`", plan.TableID, plan.CheckStaticID),
		)
		return
	}

	// Create the API request based on the plan
	var target map[string]string
	diags = plan.Params.ElementsAs(ctx, &target, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createCheckReq := anomalo.CreateCheckRequest{
		TableID:   int(plan.TableID.ValueInt64()),
		CheckType: plan.CheckType.ValueString(),
		Params:    target,
	}

	// Create new check
	var createCheckResponse *anomalo.CreateCheckResponse
	createCheckResponse, err := r.client.CreateCheck(createCheckReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Check",
			fmt.Sprintf("Could not create check, unexpected error: %s", err.Error()),
		)
		return
	}

	// Map response body back into the plan, and set the state to plan values. Some values in the plan do not
	// change based on the API response, so we do not update them.
	plan.TableID = types.Int64Value(int64(createCheckReq.TableID))
	// CheckStaticID should always match CheckID during creation. Though this is not a documented behavior, an
	// Anomalo representative indicated we could expect it.
	plan.CheckStaticID = types.Int64Value(int64(createCheckResponse.CheckID))

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *checkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state checkResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if int(state.CheckStaticID.ValueInt64()) == 0 {
		// Not expected but possible if there is a bug elsewhere in the provider.
		resp.Diagnostics.AddError("Error Reading Check",
			"Error in terraform state - a check for table ID %d has a static ID of 0. Remove it from your "+
				"state and re-import if necessary. If this issue persists notify the plugin maintainer.")
		return
	}

	// Fetch the check from Anomalo
	check, err := r.client.GetCheckByStaticID(int(state.TableID.ValueInt64()), int(state.CheckStaticID.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Checks",
			fmt.Sprintf("Could not read check for table ID %d, unexpected error: %s",
				state.TableID.ValueInt64(), err.Error()),
		)
		return
	}

	if check == nil {
		// Check was deleted remotely
		resp.State.RemoveResource(ctx)
		return
	}

	// Convert map values into terraform types.
	mapVal := map[string]attr.Value{}
	for key, val := range check.Config.Params {
		if val != nil {
			// Note: consider using safer string conversion based on runtime type of val
			// There is no "generic" terraform type analogous to interface{}, so cast to a string
			mapVal[key] = types.StringValue(fmt.Sprintf("%v", val))
		}
	}
	mapParams, diag := types.MapValue(types.StringType, mapVal)
	resp.Diagnostics.Append(diag...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Map response body back into the state, and set the response state to the updated state.
	state.CheckStaticID = types.Int64Value(int64(check.CheckStaticID))
	state.CheckType = types.StringValue(check.Config.Check)
	state.Params = mapParams

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *checkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan checkResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state checkResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !plan.CheckStaticID.IsNull() && !plan.CheckStaticID.IsUnknown() && !plan.CheckStaticID.Equal(state.CheckStaticID) {
		resp.Diagnostics.AddError(
			"Error Updating Check",
			fmt.Sprintf("Updating the check_static_id to %s for table %s is not supported. If you want to create"+
				" a new check, delete this resource and create a new one with a null or undefined check_static_id. If "+
				"you are trying to manage an existing check, import it with "+
				"`terraform import <terraform-resource-identifier> %s,%s`",
				plan.CheckStaticID.String(), plan.TableID.String(), plan.TableID.String(), plan.CheckStaticID.String()),
		)
		return
	}

	existingCheck, err := r.client.GetCheckByStaticID(int(state.TableID.ValueInt64()), int(state.CheckStaticID.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Check",
			fmt.Sprintf("Could not update check with static ID %d for table ID %d. Unable to find check to "+
				"delete. If you'd like to create this check, give it at new name and blank check_static_id. Error: %s",
				state.CheckStaticID.ValueInt64(), state.TableID.ValueInt64(), err.Error()),
		)
		return
	}

	// "Updating" checks is (confusingly) accomplished by setting check_static_id in the Params of the Check we are
	// creating. Behind the scenes, Anomalo is creating a new check with a new ID and deleting the old check.

	// Build the request to Create a new check
	var target map[string]string
	diags = plan.Params.ElementsAs(ctx, &target, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	target["check_static_id"] = strconv.Itoa(existingCheck.CheckStaticID)

	checkType := plan.CheckType.ValueString()
	createCheckReq := anomalo.CreateCheckRequest{
		TableID:   int(plan.TableID.ValueInt64()),
		CheckType: checkType,
		Params:    target,
	}

	// Create new check
	_, err = r.client.CreateCheck(createCheckReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Check",
			fmt.Sprintf("Could not create check with type %s for table %d, unexpected error: %s",
				createCheckReq.CheckType, createCheckReq.TableID, err.Error()),
		)
		return
	}

	// No need to update the plan. Plan/state values do not change based on the response of CreateCheck.
	plan.CheckStaticID = state.CheckStaticID

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *checkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var plan checkResourceModel
	diags := req.State.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	existingCheck, err := r.client.GetCheckByStaticID(int(plan.TableID.ValueInt64()), int(plan.CheckStaticID.ValueInt64()))
	if err != nil || existingCheck == nil {
		resp.Diagnostics.AddError(
			"Error Deleting Check",
			fmt.Sprintf("Error deleting check with static ID %d on table ID %d. This may be due to a race "+
				"condition. If the error persists contact the maintainer. Unexpected error: %s",
				plan.CheckStaticID.ValueInt64(), plan.TableID.ValueInt64(), err.Error()),
		)
		return
	}

	deleteRequest := anomalo.DeleteCheckRequest{
		CheckID: existingCheck.CheckStaticID,
		TableID: int(plan.TableID.ValueInt64()),
	}
	_, err = r.client.DeleteCheck(deleteRequest)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Check",
			fmt.Sprintf("Could not delete check with ID %d for table ID %d, unexpected error: %s",
				plan.CheckStaticID.ValueInt64(), plan.TableID.ValueInt64(), err.Error()),
		)
		return
	}
}

func (r *checkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	idParts := strings.Split(req.ID, ",")

	if len(idParts) != 2 || idParts[0] == "" || idParts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: tableID,checkID. Got: %q", req.ID),
		)
		return
	}

	tableID, err := strconv.Atoi(idParts[0])
	if err != nil {
		resp.Diagnostics.AddError("Error parsing table id from import identifier",
			fmt.Sprintf("Could not convert %s to int", idParts[0]))
	}
	checkID, err := strconv.Atoi(idParts[1])
	if err != nil {
		resp.Diagnostics.AddError("Error parsing check id from import identifier",
			fmt.Sprintf("Could not convert %s to int", idParts[1]))
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("table_id"), tableID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("check_static_id"), checkID)...)
}

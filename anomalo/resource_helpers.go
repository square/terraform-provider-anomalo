package anomalo

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func toPtr(str string) *string {
	if str == "" {
		return nil
	}
	return &str
}

// BoolDefaultValue PlanModifier that allows us to set a default value for undefined configuration inputs of types.Bool
func BoolDefaultValue(v types.Bool) planmodifier.Bool {
	return &boolDefaultValuePlanModifier{v}
}

type boolDefaultValuePlanModifier struct {
	DefaultValue types.Bool
}

var _ planmodifier.Bool = (*boolDefaultValuePlanModifier)(nil)

func (pm boolDefaultValuePlanModifier) Description(_ context.Context) string {
	return ""
}

func (pm boolDefaultValuePlanModifier) MarkdownDescription(_ context.Context) string {
	return ""
}

func (pm boolDefaultValuePlanModifier) PlanModifyBool(_ context.Context, req planmodifier.BoolRequest, res *planmodifier.BoolResponse) {
	// Nothing to do if the config value is set
	if !req.ConfigValue.IsNull() {
		return
	}

	res.PlanValue = pm.DefaultValue
}

// DefaultEmptyList PlanModifier that allows us to set a default value for undefined configuration inputs of types.List
func DefaultEmptyList() planmodifier.List {
	return &listDefaultValuePlanModifier{}
}

type listDefaultValuePlanModifier struct {
	DefaultValue types.List
}

var _ planmodifier.List = (*listDefaultValuePlanModifier)(nil)

func (pm listDefaultValuePlanModifier) Description(_ context.Context) string {
	return ""
}

func (pm listDefaultValuePlanModifier) MarkdownDescription(_ context.Context) string {
	return ""
}

func (pm listDefaultValuePlanModifier) PlanModifyList(_ context.Context, req planmodifier.ListRequest, res *planmodifier.ListResponse) {
	// Nothing to do if the config value is set
	if !req.ConfigValue.IsNull() {
		return
	}
	emptyList, diags := types.ListValue(types.StringType, []attr.Value{})
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	res.PlanValue = emptyList
}

// EmptyIfNull PlanModifier that converts undefined strings to the empty string ""
func EmptyIfNull() planmodifier.String {
	return &nullToEmptyStringPlanModifier{}
}

type nullToEmptyStringPlanModifier struct{}

var _ planmodifier.String = (*nullToEmptyStringPlanModifier)(nil)

func (pm nullToEmptyStringPlanModifier) Description(_ context.Context) string {
	return ""
}

func (pm nullToEmptyStringPlanModifier) MarkdownDescription(_ context.Context) string {
	return ""
}

// PlanModifyString
// This method equates null strings to empty strings. So if this modifier is applied to a Definition attribute,
//   - resource "anomalo_table" "table_name" {}
//   - resource "anomalo_table" "table_name" {definition: null}
//   - resource "anomalo_table" "table_name" {definition: ""}
//
// result in the same value for Definition.
//
// If we discover cases where anomalo differentiate between unset variables and empty strings, we will need to update
// the library accordingly.
//
// It did not seem possible to instead store strings as null, because terraform did not like it when the PlanModifier
// changed the value from "string" in the config to "null" after the modifier executes.
func (pm nullToEmptyStringPlanModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, res *planmodifier.StringResponse) {
	// Nothing to do if the config value is set
	if !req.ConfigValue.IsNull() {
		return
	}

	res.PlanValue = types.StringValue("")
}

terraform {
  required_providers {
    anomalo = {
      source = "square/anomalo"
    }
  }
}

provider "anomalo" {
  host = "https://anomalo.example.com"
  token = "<token>"
}

data "anomalo_notification_channel" "alert_channel" {
  channel_type = "slack"
  name = "#very-important-anomalo-alerts"
}

variable "alert_time" {
  type    = string
  default = "PT6H"
}
resource "anomalo_table" "VariationsTable" {
    table_name                    = "square.items.variations"
    definition                    = "All variations (deleted or active) in all merchant catalogs."
    notification_channel_id       = anomalo_notification_channel.alert_channel.id
    check_cadence_type            = "daily"
    check_cadence_run_at_duration = var.alert_time
    always_alert_on_errors        = true
}

resource "anomalo_check" "VariationsGeneratedRecently" {
    check_type      = "TimeColumnNearNow"
    table_id        = anomalo_table.VariationsTable.table_id
    params          = {
        "pass_on_no_data_error"   = "false"
        "priority_level"          = "normal"
        "time_based"              = "false"
        "time_column_target"      = "_ingested_at"
        "time_when_lag_intervals" = "0"
        "window_begin_delta"      = "-26"
        "window_end_delta"        = "0"
        "window_unit_now"         = "hours"
    }
}

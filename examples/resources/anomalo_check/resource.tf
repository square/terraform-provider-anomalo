resource "anomalo_check" "VariationsGeneratedRecently" {
    check_type      = "TimeColumnNearNow"
    table_id        = anomalo_table.VariationsTable.id
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

resource "anomalo_table" "VariationsTable" {
    table_name                    = "square.items.variations"
    definition                    = "All variations (deleted or active) in all merchant catalogs."
    notification_channel_id       = anomalo_notification_channel.alert_channel.id
    check_cadence_type            = "daily"
    check_cadence_run_at_duration = "PT6H"
    always_alert_on_errors        = true
}

data "anomalo_notification_channel" "alert_channel" {
  channel_type = "slack"
  name = "#very-important-anomalo-alerts"
}

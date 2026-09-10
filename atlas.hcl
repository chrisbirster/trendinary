variable "database_url" {
  type    = string
  default = getenv("TURSO_DATABASE_URL")
}

variable "auth_token" {
  type    = string
  default = getenv("TURSO_AUTH_TOKEN")
}

env "local" {
  url = "sqlite://trendinary.db?_fk=1"
  dev = "sqlite://atlas-dev?mode=memory&_fk=1"

  schema {
    src = "file://schema/trendinary.sql"
  }
}

env "production" {
  url = "${var.database_url}?authToken=${var.auth_token}"
  dev = "sqlite://atlas-dev?mode=memory&_fk=1"
  exclude = [
    "_litestream*",
    "trendinary_database_resets",
    "content_items.content_items_canonical_url[type=index]",
    "content_discoveries.content_discoveries_content_item_id_discovery_source_id_external_source_name[type=index]",
    "newsletter_issue_items.newsletter_issue_items_issue_id_content_item_id_section[type=index]",
    "trend_entities.trend_entities_slug[type=index]",
    "following_follows.following_follows_radar_id_kind_value[type=index]",
    "following_alerts.following_alerts_radar_id_fingerprint[type=index]",
    "following_push_subscriptions.following_push_subscriptions_radar_id_endpoint[type=index]",
  ]

  schema {
    src = "file://schema/trendinary.sql"
  }
}

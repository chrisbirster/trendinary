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
  url     = "${var.database_url}?authToken=${var.auth_token}"
  dev     = "sqlite://atlas-dev?mode=memory&_fk=1"
  exclude = ["_litestream*"]

  schema {
    src = "file://schema/trendinary.sql"
  }
}

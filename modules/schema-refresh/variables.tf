variable "schedules" {
  description = <<-DESC
    Schema refresh schedules, keyed by connection. The root module builds this
    from the refresh_schedule block on each connection, so configuration stays
    in one place even though the resource is created here.
  DESC

  type = map(object({
    connection_id = string
    cron          = string
    timezone      = string
    hard_refresh  = optional(bool, false)
  }))

  default = {}
}

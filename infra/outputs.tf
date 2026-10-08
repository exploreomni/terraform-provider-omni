output "connections" {
  value = module.warehouse.connection_ids
}

output "schema_refresh_schedules" {
  description = "Connection key to schema refresh schedule ID."
  value       = module.schema_refresh.schedule_ids
}

output "models" {
  value = module.modeling.model_ids
}

output "internal" {
  description = "Users and groups. Folders are under content, wherever they came from."
  value = {
    users  = module.access.user_ids
    groups = module.access.group_ids
  }
}

output "tenants" {
  description = <<-DESC
    What a signed URL needs for each tenant: the groups whose membership the
    sessions own, which is what manage_members = false means.

    group is the name to pass in the "groups" claim. These are non-entity
    groups; an entity group is created by the session's own entity parameter
    and is not managed here.

    content_path is deliberately absent: content usually lives in a shared hub
    folder rather than one folder per tenant, so the path depends on what you
    are linking to.
  DESC

  value = {
    for k, v in var.groups : k => {
      group                = coalesce(v.display_name, "${v.name_prefix}${k}")
      user_attribute_value = k
      routed               = contains(module.embed_routing.routed_tenants, k)
    } if !v.manage_members
  }
}

output "content" {
  description = "Folders in play, looked up and created, and who was granted what."
  value = {
    folders         = module.content_access.folder_ids
    grants          = module.content_access.grant_ids
    document_grants = module.content_access.document_grant_ids
  }
}

output "branding" {
  value = module.branding.palette_ids
}

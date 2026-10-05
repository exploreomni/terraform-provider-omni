output "connections" {
  value = module.warehouse.connection_ids
}

output "models" {
  value = module.modeling.model_ids
}

output "internal" {
  description = "Internal users, groups and folders. Empty on an embed-only instance."
  value = {
    users   = module.access.user_ids
    groups  = module.access.group_ids
    folders = module.access.folder_ids
  }
}

output "tenants" {
  description = <<-DESC
    What a signed URL needs for each tenant. content_path is deliberately not
    here: content usually lives in a shared hub folder rather than one folder
    per tenant, so the path depends on what you are linking to.
  DESC

  value = {
    for k, name in module.embed_groups.group_names : k => {
      group                = name
      user_attribute_value = k
      routed               = contains(module.embed_routing.routed_tenants, k)
    }
  }
}

output "content" {
  description = "Folders in play, looked up and created, and who was granted what."
  value = {
    folders = module.content_access.folder_ids
    grants  = module.content_access.grant_ids
  }
}

output "branding" {
  value = module.branding.palette_ids
}

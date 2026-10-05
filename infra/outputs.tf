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
  description = "What a signed URL needs for each tenant. Empty on an internal-only instance."
  value       = { for k, m in module.tenant : k => m.signed_url_values }
}

output "branding" {
  value = module.branding.palette_ids
}

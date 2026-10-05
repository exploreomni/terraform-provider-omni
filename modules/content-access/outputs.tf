output "folder_ids" {
  description = "Local name to folder ID, for folders looked up and created."
  value       = local.folder_ids
}

output "grant_ids" {
  value = { for k, p in omni_folder_permission.this : k => p.id }
}

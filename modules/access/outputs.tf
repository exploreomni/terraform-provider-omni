output "user_ids" {
  value = { for k, u in omni_user.this : k => u.id }
}

output "group_ids" {
  value = { for k, g in omni_user_group.this : k => g.id }
}

output "folder_ids" {
  value = { for k, f in omni_folder.this : k => f.id }
}

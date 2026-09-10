# IDs of everything under management, so downstream configs and humans can find
# resources without clicking through the UI.

output "folder_ids" {
  description = "Folder name to ID."
  value = {
    reporting = omni_folder.reporting.id
    finance   = omni_folder.finance.id
  }
}

output "folders" {
  description = "Full folder detail, including paths and UI links."
  value = {
    for k, f in {
      reporting = omni_folder.reporting
      finance   = omni_folder.finance
      } : k => {
      id    = f.id
      name  = f.name
      path  = f.path
      url   = f.url
      scope = f.scope
    }
  }
}

output "user_group_ids" {
  description = "User group name to ID."
  value = {
    finance = omni_user_group.finance.id
  }
}

resource "omni_folder_permission" "analysts_view_finance" {
  folder_id = omni_folder.finance.id
  role      = "VIEWER"

  # Authoritative for this resource: a group removed from the set loses the
  # grant on the next apply.
  user_group_ids = [omni_user_group.analysts.id]
}

# One resource per role. Two groups with different roles on the same folder
# are two resources, because the role is part of this resource's identity and
# changing it forces a replacement.
resource "omni_folder_permission" "finance_team_edits_finance" {
  folder_id      = omni_folder.finance.id
  role           = "EDITOR"
  user_group_ids = [omni_user_group.finance_team.id]
}

# OWNER can only be granted to users, never to groups.
resource "omni_folder_permission" "finance_owner" {
  folder_id = omni_folder.finance.id
  role      = "OWNER"
  user_ids  = [omni_user.head_of_finance.id]
}

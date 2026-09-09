resource "omni_user_group" "finance" {
  display_name = "Finance"

  member_ids = [
    omni_user.blob.id,
  ]
}

resource "omni_document_permission" "board_pack_viewers" {
  document_id = "abc123de"
  role        = "VIEWER"

  user_group_ids = [omni_user_group.executives.id]
}

# Like folder permissions, one resource per role, and the sets are
# authoritative for the role they name.
resource "omni_document_permission" "board_pack_editors" {
  document_id = "abc123de"
  role        = "EDITOR"

  user_ids = [
    omni_user.head_of_finance.id,
    omni_user.chief_of_staff.id,
  ]
}

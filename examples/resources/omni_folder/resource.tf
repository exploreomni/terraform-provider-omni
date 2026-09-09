resource "omni_folder" "finance" {
  name  = "Finance"
  path  = "finance"
  scope = "organization"
}

resource "omni_folder" "monthly_reporting" {
  name             = "Monthly reporting"
  parent_folder_id = omni_folder.finance.id
}

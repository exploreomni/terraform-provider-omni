# Folders and who can see them. scope is stated, not left to the provider
# default, because it decides who can reach a folder before any grant.

existing_folders = {}

# Two folders, chosen to exercise both depths. managed_folders supports one
# level of nesting, and top-level and nested folders are separate resources in
# the module because a single resource pointing at itself is a cycle Terraform
# rejects.
managed_folders = {
  "shared-hub" = {
    name = "Shared Hub"

    # Org-wide. The grant below adds a role on top; scope decides who can reach
    # the folder at all before that.
    scope = "organization"
  }

  # Nested. Inherits the parent's scope and must not state one.
  "shared-hub/acme" = {
    name   = "Acme"
    parent = "shared-hub"
  }
}

content_grants = {
  "hub-analysts" = {
    folder = "shared-hub"
    role   = "VIEWER"
    groups = ["analysts"]
  }
}

# Grants on an individual document. Empty: prefer a folder grant, and this needs
# a document ID from the UI, since documents are not created by Terraform.
document_grants = {}

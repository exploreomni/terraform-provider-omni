# Folders, and who can see them.
#
# var.folders says which folders exist. content_grants and document_grants say
# who can see them. One question each.

folders = {
  # Created, top-level. scope is stated rather than left to the provider
  # default, because it decides who can reach the folder before any grant.
  "shared-hub" = {
    name  = "Shared Hub"
    scope = "organization"
  }

  # Created, nested. Inherits the parent's scope and must not state one.
  "shared-hub/acme" = {
    name   = "Acme"
    parent = "shared-hub"
  }

  # Created, for the internal team. No grant here; see content_grants below.
  "analytics" = {
    name  = "Analytics"
    scope = "organization"
  }

  # A looked-up folder would be:
  #
  #   "hub" = { existing_path = "Shared/Hub" }
  #
  # which fails at plan time if it is missing, rather than creating a second
  # folder beside the real one.
}

content_grants = {
  "hub-analysts" = {
    folder = "shared-hub"
    role   = "VIEWER"
    groups = ["analysts"]
  }

  "analytics-analysts" = {
    folder = "analytics"
    role   = "EDITOR"
    groups = ["analysts"]
  }
}

# Grants on an individual document. Empty: prefer a folder grant, and this needs
# a document ID from the UI, since documents are not created by Terraform.
document_grants = {}

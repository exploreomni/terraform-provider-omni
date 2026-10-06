# Folders and who can see them.
#
# Shared content lives in a hub folder created by a person, so the usual shape
# is a lookup plus a grant. Terraform creates a folder only where content
# belongs to one group and would not sit in the hub.

# Folders that already exist. A missing one fails at plan time rather than
# being silently recreated.
existing_folders = {}

# Folders to create. Empty is a perfectly good answer.
managed_folders = {
  hub = { name = "Shared analytics" }
}

# Who can see what. Several tenants on one grant is normal: shared content is
# shared.
content_grants = {
  "hub-for-tenants" = {
    folder        = "hub"
    role          = "VIEWER"
    tenant_groups = ["acme", "globex"]
  }
}

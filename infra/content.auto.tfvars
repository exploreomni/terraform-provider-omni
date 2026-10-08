# Folders and who can see them.
#
# Shared content lives in a hub folder, and the usual shape is one folder plus
# one grant per audience.

# Folders that already exist. A missing one fails at plan time rather than
# being silently recreated.
existing_folders = {}

# Folders to create.
#
# scope is stated, not inherited from the provider default. It decides who can
# reach the folder before any grant below applies:
#
#   organization  org-level content, with grants layering roles on top
#   restricted    reachable only through a grant, and needs owner_id when the
#                 provider uses an organization API key
managed_folders = {
  hub = {
    name  = "Shared analytics"
    scope = "organization"
  }
}

# Who can see what. Several tenants on one grant is normal: shared content is
# shared. A grant must name at least one group or user.
content_grants = {
  "hub-for-tenants" = {
    folder        = "hub"
    role          = "VIEWER"
    tenant_groups = ["acme", "globex"]
  }
}

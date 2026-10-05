# Colour palettes and the label taxonomy.
#
# Per-tenant palettes are declared in tenants.auto.tfvars and merged in
# automatically. This file is for palettes that are not tied to a tenant.

palettes = {}

labels = {
  "verified" = {
    color       = "#0366d6"
    description = "Reviewed and trusted content"
  }

  "deprecated" = {
    color       = "#d73a49"
    description = "Scheduled for removal"
  }
}

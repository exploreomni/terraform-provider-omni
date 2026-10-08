# Colour palettes and the label taxonomy.
#
# Per-tenant palettes are declared in tenants.auto.tfvars and merged in
# automatically. This file is for palettes not tied to a tenant.

palettes = {}

# "verified" is one of Omni's built-in labels, so creating it returns 409.
# Either use a name Omni does not ship, as here, or import the existing one:
#
#   terraform import 'module.branding.omni_label.this["verified"]' verified
labels = {
  "reviewed" = {
    color       = "#0366d6"
    description = "Reviewed and trusted content"
  }

  "deprecated" = {
    color       = "#d73a49"
    description = "Scheduled for removal"
  }
}

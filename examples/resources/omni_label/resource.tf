resource "omni_label" "certified" {
  name        = "Certified"
  color       = "#0366d6"
  description = "Reviewed by the data team. Numbers here can be quoted externally."

  # Both of these need an admin token. A non-admin gets a 403.
  homepage = true
  verified = true
}

resource "omni_label" "draft" {
  name        = "Draft"
  color       = "#e36209"
  description = "Work in progress. Do not build on these."
}

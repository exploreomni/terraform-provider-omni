# Colour palettes and the label taxonomy.
#
# Labels here means the labels that exist, not their use. Applying a label to a
# document is part of authoring that document and stays in the UI.

terraform {
  required_providers {
    omni = {
      source = "ashima-omni/omni"
    }
  }
}

resource "omni_color_palette" "this" {
  for_each = var.palettes

  name   = each.key
  type   = each.value.type
  colors = each.value.colors
}

resource "omni_label" "this" {
  for_each = var.labels

  name        = each.key
  color       = each.value.color
  description = each.value.description
}

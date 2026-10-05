output "palette_ids" {
  description = "Palette name to ID. Visualisations reference a palette by ID."
  value       = { for k, p in omni_color_palette.this : k => p.id }
}

output "label_names" {
  value = [for l in omni_label.this : l.name]
}

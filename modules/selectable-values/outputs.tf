output "ids" {
  description = "Local key to the composite ID, in the form <attribute id>:group:<id>."
  value       = { for k, s in omni_user_attribute_selectable_values.this : k => s.id }
}

output "values_by_user" {
  description = "Email to a map of attribute name to value, for omni_user.attributes."
  value       = local.values_by_user
}

output "names" {
  description = "Local key to the attribute's current name."
  value       = local.names
}

output "ids" {
  description = "Local key to the definition's ID, whether created or referenced."
  value       = local.ids
}

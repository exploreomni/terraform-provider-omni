output "values_by_user" {
  description = <<-DESC
    Email to a map of attribute name to value, ready to merge into the users
    passed to modules/access. Names are resolved from IDs here so configuration
    never has to carry one.
  DESC
  value       = local.values_by_user
}

output "names" {
  description = "Local key to the attribute's current name in Omni."
  value       = local.names
}

output "ids" {
  description = "Local key to the attribute's ID, echoed back for convenience."
  value       = { for k, a in data.omni_user_attribute.this : k => a.id }
}

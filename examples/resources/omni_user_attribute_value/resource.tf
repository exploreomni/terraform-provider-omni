# A person Terraform does not own: they signed in on their own, so look them
# up instead of declaring an omni_user, which would deprovision them on destroy.
data "omni_user" "store_manager" {
  user_name = "store.manager@example.com"
}

# The definition can live in Terraform too.
resource "omni_user_attribute" "location_id" {
  name            = "location_id"
  label           = "Location"
  type            = "String"
  multiple_values = true
}

# The stores this person may see. Topic access filters key off location_id.
resource "omni_user_attribute_value" "store_manager_locations" {
  user_id = data.omni_user.store_manager.id
  name    = omni_user_attribute.location_id.name
  values  = ["123", "456"]
}

# A single-valued attribute takes value instead of values.
resource "omni_user_attribute_value" "store_manager_region" {
  user_id = data.omni_user.store_manager.id
  name    = "region"
  value   = "anz"
}

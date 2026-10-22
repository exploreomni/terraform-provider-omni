resource "omni_user_attribute" "region" {
  # Reference name, used in model SQL and in embed SSO URL parameters. Letters,
  # numbers and underscores, starting with a letter, and never "omni_".
  name = "region"

  # Display name in the UI. Must not start with "Omni", and must not be one of
  # the reserved connection labels such as Host, Database or Schema.
  label       = "Region"
  description = "Sales region, used by the access filter on the orders topic."

  type          = "String"
  default_value = "anz"
}

resource "omni_user_attribute" "cost_centres" {
  name            = "cost_centres"
  label           = "Cost centres"
  type            = "String"
  multiple_values = true

  # Use default_values with multiple_values, and default_value without.
  # Setting both is rejected at plan time.
  default_values = ["unassigned"]
}

resource "omni_user_attribute" "spend_limit" {
  name  = "spend_limit"
  label = "Spend limit"
  type  = "Number"

  # Numbers are given as strings so that values beyond 2^53 - 1 keep their
  # precision, which is also how Omni stores them.
  default_value = "25000"
}

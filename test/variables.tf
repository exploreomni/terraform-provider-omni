# Values that change between the create and update phases. The runner script
# flips them, so the same config exercises both paths without editing files.

variable "suffix" {
  description = "Appended to every resource name. Change it to drive updates."
  type        = string
  default     = "v1"
}

variable "connection_name" {
  description = "An existing connection to build the test model on."
  type        = string
}

variable "base_model_name" {
  description = "An existing SHARED model to extend."
  type        = string
}

variable "test_user_email" {
  description = "Email for the test user. Must not belong to a real person."
  type        = string
  default     = "tf-provider-test@example.invalid"
}

variable "role_name" {
  description = "Role assigned in the role tests. Changed to drive the update."
  type        = string
  default     = "QUERIER"
}

variable "user_attribute_key" {
  description = "Reference of an existing user attribute, or empty to skip attribute testing."
  type        = string
  default     = ""
}

variable "topic_description" {
  description = "Written into the test topic YAML. Changed to drive the update."
  type        = string
  default     = "Created by the Terraform provider test suite."
}

variable "test_connection" {
  description = "Whether to exercise omni_connection. Needs real warehouse credentials."
  type        = bool
  default     = false
}

variable "connection_dialect" {
  type    = string
  default = "postgres"
}

variable "connection_host" {
  type    = string
  default = ""
}

variable "connection_database" {
  type    = string
  default = ""
}

variable "connection_username" {
  type    = string
  default = ""
}

variable "connection_password" {
  type      = string
  default   = ""
  sensitive = true
}

variable "connection_base_role" {
  description = "Changed to drive the in-place connection update."
  type        = string
  default     = "NO_ACCESS"
}

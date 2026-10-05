variable "palettes" {
  description = <<-DESC
    Colour palettes, keyed by name. Colour order matters: discrete palettes
    assign them in sequence, continuous palettes interpolate between adjacent
    values.
  DESC

  type = map(object({
    type   = string
    colors = list(string)
  }))

  default = {}
}

variable "labels" {
  description = "Content labels, keyed by name."
  type = map(object({
    color       = optional(string)
    description = optional(string)
  }))
  default = {}
}

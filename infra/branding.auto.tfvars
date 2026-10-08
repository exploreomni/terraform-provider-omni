# Palettes and labels. "verified" is one of Omni's built-in labels, so
# creating it returns 409; "reviewed" is used instead.

palettes = {
  "brand" = {
    type   = "discrete"
    colors = ["#1f77b4", "#ff7f0e", "#2ca02c", "#d62728", "#9467bd"]
  }
}

labels = {
  "reviewed" = {
    color       = "#0366d6"
    description = "Reviewed and trusted content"
  }

  "deprecated" = {
    color       = "#d73a49"
    description = "Scheduled for removal"
  }
}

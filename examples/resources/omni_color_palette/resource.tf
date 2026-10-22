resource "omni_color_palette" "brand_categorical" {
  name = "Brand categorical"
  type = "discrete"

  # Order matters: a discrete palette assigns these to categories in sequence.
  colors = [
    "#1f77b4",
    "#ff7f0e",
    "#2ca02c",
    "#d62728",
    "#9467bd",
  ]
}

resource "omni_color_palette" "revenue_gradient" {
  name = "Revenue gradient"
  type = "continuous"

  # A continuous palette interpolates between adjacent colours, so two or
  # three stops are usually enough.
  colors = [
    "#f7fbff",
    "#6baed6",
    "#08306b",
  ]
}

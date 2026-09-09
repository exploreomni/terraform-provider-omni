resource "omni_user" "blob" {
  user_name    = "blob.ross@blobsrus.co"
  display_name = "Blob Ross"

  attributes = {
    region     = "APAC"
    department = "Finance"
  }
}

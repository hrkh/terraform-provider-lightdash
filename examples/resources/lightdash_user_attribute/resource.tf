resource "lightdash_group" "sales_us" {
  organization_uuid = "xxxxxxxx-xxxxxxxxxx-xxxxxxxxx"
  name              = "sales_us"
  members           = []
}

resource "lightdash_group" "sales_emea" {
  organization_uuid = "xxxxxxxx-xxxxxxxxxx-xxxxxxxxx"
  name              = "sales_emea"
  members           = []
}

# A user attribute with values assigned to groups
resource "lightdash_user_attribute" "region" {
  name        = "region"
  description = "The sales regions that a user can see"

  # Used for users who don't have their own value
  attribute_defaults = ["none"]

  groups = [
    {
      group_uuid = lightdash_group.sales_us.group_uuid
      values     = ["us"]
    },
    {
      group_uuid = lightdash_group.sales_emea.group_uuid
      values     = ["uk", "de", "fr"]
    },
  ]

  # Values can be assigned to individual users too
  users = [
    {
      user_uuid = "xxxxxxxxxxx-xxxxxxxxxxxx-xxxxxxxxxx"
      values    = ["us", "uk"]
    },
  ]
}

# A user attribute without any values
resource "lightdash_user_attribute" "is_admin" {
  name = "is_admin"
}

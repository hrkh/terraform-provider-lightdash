Manages a Lightdash user attribute in the organization, together with the values assigned to groups and users. User attributes are referenced by name from dbt, for example `required_attributes` to restrict access to dimensions and tables, and `${lightdash.attributes.<name>}` in `sql_filter` for row-level filtering.

This resource is authoritative for the whole user attribute. The Lightdash API replaces the definition, the group values and the user values at once, so any value assigned outside of Terraform, such as in the Lightdash UI, is removed on the next apply. Manage each user attribute with a single `lightdash_user_attribute` resource.

Lightdash combines the values that a user gets from `users` and from the `groups` they belong to. See the [Lightdash documentation](https://docs.lightdash.com/workspace-admin/user-attributes) for how the values and `attribute_defaults` are resolved.

It requires Lightdash 0.3243.0 or later, which added multiple values per user attribute. The resource always sends the `values` and `attributeDefaults` fields of the API, which older versions don't have.

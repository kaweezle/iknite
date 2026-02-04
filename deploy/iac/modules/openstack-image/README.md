# OpenStack Image Module

Creates OpenStack images from local files and exposes their IDs for reuse in
other infrastructure. Images are automatically recreated when source files
change, detected via SHA256 hash comparison.

## How to use

```hcl
module "images" {
  source = "../modules/openstack-image"

  openstack = {
    auth_url             = var.openstack_auth_url
    user_name            = var.openstack_user_name
    password             = var.openstack_password
    user_domain_name     = "Default"
    project_domain_name  = "Default"
    tenant_id            = var.openstack_project_id
    region               = "GRA"
  }

  images = {
    "my-image" = {
      name              = "my-custom-image"
      local_file_path   = "${path.root}/images/my-image.qcow2"
      container_format  = "bare"
      disk_format       = "qcow2"
      visibility        = "private"
    }
  }
}
```

<!-- markdownlint-disable -->
<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 1.11.0 |
| <a name="requirement_null"></a> [null](#requirement\_null) | 3.2.4 |
| <a name="requirement_openstack"></a> [openstack](#requirement\_openstack) | 3.4.0 |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [null_resource.image_hash](https://registry.terraform.io/providers/hashicorp/null/3.2.4/docs/resources/resource) | resource |
| [openstack_images_image_v2.this](https://registry.terraform.io/providers/terraform-provider-openstack/openstack/3.4.0/docs/resources/images_image_v2) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_images"></a> [images](#input\_images) | Map of images to create | <pre>map(object({<br/>    container_format = optional(string, "bare")<br/>    disk_format      = optional(string, "qcow2")<br/>    local_file_path  = string<br/>    name             = string<br/>    visibility       = optional(string, "private")<br/>  }))</pre> | n/a | yes |
| <a name="input_openstack"></a> [openstack](#input\_openstack) | OpenStack credentials | <pre>object({<br/>    auth_url            = string<br/>    password            = string<br/>    project_domain_name = string<br/>    region              = string<br/>    tenant_id           = string<br/>    user_domain_name    = string<br/>    user_name           = string<br/>  })</pre> | n/a | yes |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_images"></a> [images](#output\_images) | Details of images created in OpenStack keyed by input map key. |
<!-- END_TF_DOCS -->
<!-- markdownlint-enable -->

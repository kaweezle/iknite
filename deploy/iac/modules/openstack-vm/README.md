# OpenStack VM Module

Provisions OpenStack compute instances from pre-existing images. Images must be
supplied externally (typically via the openstack-image module). Optionally
creates DNS records and manages SSH keypairs for instance access.

## How to use

```hcl
module "vms" {
  source = "../modules/openstack-vm"

  openstack = {
    auth_url             = var.openstack_auth_url
    user_name            = var.openstack_user_name
    password             = var.openstack_password
    user_domain_name     = "Default"
    project_domain_name  = "Default"
    tenant_id            = var.openstack_project_id
    region               = "GRA"
  }

  ovh = {
    endpoint            = "ovh-eu"
    application_key     = var.ovh_app_key
    application_secret  = var.ovh_app_secret
    consumer_key        = var.ovh_consumer_key
  }

  keys = {
    "default" = {
      name       = "my-keypair"
      public_key = file("~/.ssh/id_rsa.pub")
    }
  }

  private_keys = {
    "default" = file("~/.ssh/id_rsa")
  }

  instances = {
    "my-vm" = {
      name         = "my-vm-instance"
      image_id     = module.images.images["my-image"].id
      flavor_name  = "m1.small"
      key_name     = "default"
      dns_zone     = "example.com"
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
| <a name="requirement_ovh"></a> [ovh](#requirement\_ovh) | 2.10.0 |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [null_resource.wait_ssh](https://registry.terraform.io/providers/hashicorp/null/3.2.4/docs/resources/resource) | resource |
| [openstack_compute_instance_v2.this](https://registry.terraform.io/providers/terraform-provider-openstack/openstack/3.4.0/docs/resources/compute_instance_v2) | resource |
| [openstack_compute_keypair_v2.this](https://registry.terraform.io/providers/terraform-provider-openstack/openstack/3.4.0/docs/resources/compute_keypair_v2) | resource |
| [ovh_domain_zone_record.this](https://registry.terraform.io/providers/ovh/ovh/2.10.0/docs/resources/domain_zone_record) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_instances"></a> [instances](#input\_instances) | Map of instances to create. image\_name references a key in var.images. | <pre>map(object({<br/>    dns_zone     = optional(string)<br/>    enabled      = optional(bool, true)<br/>    flavor_name  = string<br/>    image_id     = string<br/>    key_name     = string<br/>    metadata     = optional(map(string), {})<br/>    name         = string<br/>    network_name = optional(string, "Ext-Net")<br/>    tags         = optional(list(string), [])<br/>    ttl          = optional(number, 60)<br/>    user_data    = optional(string, null)<br/>  }))</pre> | n/a | yes |
| <a name="input_keys"></a> [keys](#input\_keys) | Map of keypairs to create | <pre>map(object({<br/>    name       = string<br/>    public_key = string<br/>  }))</pre> | n/a | yes |
| <a name="input_openstack"></a> [openstack](#input\_openstack) | OpenStack credentials | <pre>object({<br/>    auth_url            = string<br/>    user_domain_name    = string<br/>    project_domain_name = string<br/>    user_name           = string<br/>    password            = string<br/>    region              = string<br/>    tenant_id           = string<br/>  })</pre> | n/a | yes |
| <a name="input_ovh"></a> [ovh](#input\_ovh) | OVH credentials | <pre>object({<br/>    endpoint           = optional(string, "ovh-eu")<br/>    application_key    = string<br/>    application_secret = string<br/>    consumer_key       = string<br/>  })</pre> | n/a | yes |
| <a name="input_private_keys"></a> [private\_keys](#input\_private\_keys) | Map of private keys for the created keypairs | `map(string)` | n/a | yes |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_instances"></a> [instances](#output\_instances) | Alpine instance IP address |
| <a name="output_keypairs"></a> [keypairs](#output\_keypairs) | Keypairs created |
<!-- END_TF_DOCS -->
<!-- markdownlint-enable -->

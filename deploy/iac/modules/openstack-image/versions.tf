terraform {
  required_version = ">= 1.11.0"

  required_providers {
    null = {
      source  = "hashicorp/null"
      version = "3.2.4"
    }

    openstack = {
      source  = "terraform-provider-openstack/openstack"
      version = "3.4.0"
    }
  }
}

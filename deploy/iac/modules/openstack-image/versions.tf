terraform {
  required_version = ">= 1.11.0"

  required_providers {
    null = {
      source  = "hashicorp/null"
      version = "3.3.0"
    }

    openstack = {
      source  = "terraform-provider-openstack/openstack"
      version = "3.4.0"
    }
  }
}

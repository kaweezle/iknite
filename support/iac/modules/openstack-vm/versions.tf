terraform {
  required_version = ">= 1.11.0"
  required_providers {
    openstack = {
      source  = "terraform-provider-openstack/openstack"
      version = "3.4.0"
    }
    ovh = {
      source  = "ovh/ovh"
      version = "2.10.0"
    }
    null = {
      source  = "hashicorp/null"
      version = "3.2.4"
    }
  }
}

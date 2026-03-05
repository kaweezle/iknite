terraform {
  required_version = ">= 1.11.0"
  required_providers {
    openstack = {
      source  = "terraform-provider-openstack/openstack"
      version = "3.0.0"
    }
    ovh = {
      source  = "ovh/ovh"
      version = "2.11.0"
    }
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.34"
    }
  }
}

terraform {
  required_version = ">= 1.11.0"

  required_providers {
    null = {
      source  = "hashicorp/null"
      version = "3.3.0"
    }

    incus = {
      source  = "lxc/incus"
      version = "1.1.0"
    }
  }
}

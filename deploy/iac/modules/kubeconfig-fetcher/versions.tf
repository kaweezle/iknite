terraform {
  required_version = ">= 1.14"

  required_providers {
    null = {
      source  = "hashicorp/null"
      version = "~> 3.0"
    }
  }
}

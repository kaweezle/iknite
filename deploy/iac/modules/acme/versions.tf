terraform {
  required_version = ">= 1.11.0"
  required_providers {
    acme = {
      source  = "vancluever/acme"
      version = ">= 2.42.0"
    }

    tls = {
      source  = "hashicorp/tls"
      version = ">= 4.0.5"
    }
  }
}

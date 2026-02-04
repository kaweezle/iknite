terraform {
  required_version = ">= 1.14"

  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 3.0.1"
    }
    null = {
      source  = "hashicorp/null"
      version = "~> 3.0"
    }
    local = {
      source  = "hashicorp/local"
      version = "~> 2.6.2"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.8.1"
    }

  }
}

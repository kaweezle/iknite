terraform {
  required_version = ">= 1.11"

  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 3.1.0"
    }
    null = {
      source  = "hashicorp/null"
      version = "~> 3.0"
    }
    helmfile = {
      source  = "kaweezle/helmfile"
      version = "~> 0.1.2"
    }
    local = {
      source  = "hashicorp/local"
      version = "~> 2.8.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.8.1"
    }
  }
}

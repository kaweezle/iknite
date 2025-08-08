config {
  format = "compact"
  module = true
}

plugin "terraform" {
  enabled = true
  preset  = "all"

  version = "0.5.0"
  source  = "github.com/terraform-linters/tflint-ruleset-terraform"
}

plugin "azurerm" {
  enabled = true
  version = "0.25.1"
  source  = "github.com/terraform-linters/tflint-ruleset-azurerm"
}

rule "terraform_unused_required_providers" {
  enabled = false
}

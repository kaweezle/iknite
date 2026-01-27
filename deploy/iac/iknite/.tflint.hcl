config {
  format           = "compact"
  call_module_type = "local"
}

plugin "terraform" {
  enabled = true
  preset  = "all"

  version = "0.14.1"
  source  = "github.com/terraform-linters/tflint-ruleset-terraform"
}

plugin "azurerm" {
  enabled = true
  version = "0.30.0"
  source  = "github.com/terraform-linters/tflint-ruleset-azurerm"
}

rule "terraform_unused_required_providers" {
  enabled = false
}

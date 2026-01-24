---
description: 'Terraform Conventions and Guidelines'
applyTo: 'deploy/iac/**/*'
---

<!-- cSpell: words kwzltfstate earlyvalue2 tftest -->

# Terraform and Terragrunt Conventions

## Overview

This document defines conventions and best practices for Terraform modules and
Terragrunt configurations in this project. All configurations should adhere to
these guidelines to ensure consistency, maintainability, and security.

### General Principles

- Use Terragrunt with Terraform to provision and manage infrastructure
- Use version control for all configurations
- Prioritize readability, clarity, and maintainability
- Use a shared plugin cache directory to avoid provider duplication:
  ```bash
  export TF_PLUGIN_CACHE_DIR="$HOME/.cache/terraform/plugin-cache"
  ```

## Project Structure

### Directory Organization

- **Modules**: Located in [deploy/iac/modules](../../deploy/iac/modules) -
  Reusable Terraform modules that encapsulate common patterns
- **Terragrunt Units**: Located in
  [deploy/iac/iknite](../../deploy/iac/iknite) - Configuration units that
  consume modules

### Module Structure

All modules must have the following files:

- `main.tf`: Primary resource definitions
- `variables.tf`: Input variable definitions
- `outputs.tf`: Output value definitions
- `versions.tf`: Required Terraform and provider versions
- `providers.tf`: Provider configurations
- `README.md`: Documentation (generated with `terraform-docs`)

## Terragrunt Conventions

### Root Configuration

The terragrunt root directory [deploy/iac/iknite](../../deploy/iac/iknite)
contains [`root.hcl`](../../deploy/iac/iknite/root.hcl), which:

- Defines common configuration (remote state, providers)
- Is included by all terragrunt units:
  ```hcl
  include "root" {
    path   = find_in_parent_folders("root.hcl")
    expose = true
  }
  ```

### Remote State Management

- `root.hcl` generates `backend.tf` during `terragrunt init`
- State files are stored in S3 bucket `kwzltfstate` with keys following the
  pattern:
  ```
  iknite/${path_relative_to_include()}/terraform.tfstate
  ```
- Example: `deploy/iac/iknite/apkrepo` → `iknite/apkrepo/terraform.tfstate`

### Module References

Units reference modules by path from the repo root:

```hcl
# deploy/iac/iknite/public-object-store/terragrunt.hcl
terraform {
  source = "${get_repo_root()}/deploy/iac/modules/public-object-store"
}
```

### Shared Variables

`root.hcl` defines common variables in its `locals` section:

```hcl
locals {
  iknite_version     = include.root.locals.iknite_version
  kubernetes_version = include.root.locals.kubernetes_version
}
```

### Provider Configuration

- `root.hcl` can generate `providers.tf` and `versions.tf` if they don't exist
- **Best practice**: Define them explicitly in each module for clarity and
  portability

### Sub-configurations

Sub-folders can contain a `sub.hcl` file for shared configuration:

```hcl
// deploy/iac/iknite/dev/sub.hcl
locals {
  environment = "dev"
}

// deploy/iac/iknite/dev/app1/terragrunt.hcl
include "sub" {
  path   = find_in_parent_folders("sub.hcl")
  expose = true
}
```

### Dependencies

Terragrunt units form a Directed Acyclic Graph (DAG):

```hcl
dependency "certs" {
  config_path = "${get_parent_terragrunt_dir("root")}/acme"
}

inputs = {
  load_balancers = {
    "iknite-lb" = {
      listeners = {
        https = {
          tls_certificate_p12 = dependency.certs.outputs.pfx["all-iknite-app"]
        }
      }
    }
  }
}
```

## Terraform Module Conventions

### Resource Naming

- **Preferred resource name**: `this`
  ```terraform
  resource "aws_instance" "this" {
    for_each = var.instances
    ...
  }
  ```
- Use descriptive names for resources, variables, and outputs
- Use consistent naming conventions across all configurations

### Resource Creation Patterns

- **Use `for_each` over `count`** for better readability and maintainability
- Use `for_each` with maps to create multiple instances of a resource
- Use `count` only for numeric iterations

### Nested Resources

Resources associated with primary resources should be nested in the main
variable:

```terraform
// variables.tf
variable "load_balancers" {
  type = map(object({
    name            = string
    flavor_name     = string
    listeners = optional(map(object({
      protocol            = string
      protocol_port       = number
    })), {})
  }))
}

// main.tf
resource "openstack_lb_loadbalancer_v2" "this" {
  for_each = var.load_balancers
  name     = each.value.name
  ...
}

locals {
  // flatten lb listeners into a single map for resource creation
  lb_listeners = { for item in flatten([
    for lb_key, lb in var.load_balancers : [
      for listener_key, listener in lb.listeners :
        merge(listener, { lb_key = lb_key, listener_key = listener_key })
    ]
  ]) : "${item.lb_key}-${item.listener_key}" => item }
}

resource "openstack_lb_listener_v2" "this" {
  for_each        = local.lb_listeners
  name            = each.key
  loadbalancer_id = openstack_lb_loadbalancer_v2.this[each.value.lb_key].id
  ...
}
```

## Variables and Inputs

### Variable Design

- **Preferred type**: `map(object{...})` to group related values
- Map keys represent resource names
- Object properties represent resource attributes

Example:

```terraform
variable "instances" {
  type = map(object({
    name         = string
    enabled      = optional(bool, true)
    image_name   = string
  }))
}
```

### Variable Guidelines

- Each module should have one main map variable defining primary resources
- Include a `name` attribute when resource name constraints exist
- Use `enabled` boolean sparingly - only when resources need frequent
  creation/destruction
- Set default values where appropriate
- Group all credentials for a provider in a single `map(string)` variable
- Always include `description` and `type` attributes

### Using Data Sources

- Retrieve information about existing resources instead of manual configuration
- This ensures configurations are always up-to-date and environment-adaptive
- Avoid data sources for resources created in other terragrunt units (use
  outputs and dependencies instead)
- Remove unnecessary data sources to improve performance

### Using Locals

- Use `locals` for values used multiple times to ensure consistency
- Use `locals` to compute derived values or flatten nested structures

## Security

### Version Management

- Always use the latest stable version of Terraform and its providers
- Regularly update configurations to incorporate security patches and
  improvements

### Sensitive Data Management

- **Never commit sensitive information** to version control:
  - AWS credentials, API keys, passwords
  - Certificates, private keys
  - Terraform state files
- Use `.gitignore` to exclude sensitive files
- Store sensitive information in SOPS-encrypted files:
  - Format: YAML
    ([deploy/iac/iknite/secrets.sops.yaml](../../deploy/iac/iknite/secrets.sops.yaml))
  - Use `sops` to encrypt/decrypt
  - Load in `root.hcl`:
    ```hcl
    locals {
      secret = yamldecode(sops_decrypt_file(find_in_parent_folders("secrets.sops.yaml")))
    }
    ```
  - Reference in units:
    ```hcl
    locals {
      secret = include.root.locals.secret
    }
    ```

### Variable and Output Security

- **Always mark sensitive variables** as `sensitive = true`
- This prevents values from appearing in plan/apply output
- Separate sensitive and non-sensitive outputs when needed
- Avoid outputting entire resource objects - output only necessary attributes:
  ```terraform
  output "files" {
    value = { for k, v in aws_s3_object.this : k => {
      bucket_name  = v.bucket
      key          = v.key
      etag         = v.etag
      content_type = v.content_type
    } }
    description = "The files uploaded to the S3 bucket."
  }
  ```

### Network Security

- Use security groups and network ACLs to control network access
- Apply principle of least privilege to resource permissions

### Security Scanning

- Regularly audit configurations for vulnerabilities
- Use security scanning tools:
  - `tfsec`
  - `checkov`
  - `trivy`

## Code Organization and Maintainability

### Terragrunt Unit Organization

Use separate terragrunt folders for each major infrastructure component.
Benefits:

- Reduces complexity
- Easier to manage and maintain
- Faster `plan` and `apply` operations
- Independent development and deployment
- Reduces risk of accidental changes to unrelated resources

### Module Design Principles

- Use modules to avoid configuration duplication
- Encapsulate related resources and configurations
- Simplify complex configurations and improve readability
- **Avoid**:
  - Circular dependencies between modules
  - Unnecessary layers of abstraction
  - Modules for single resources (only for groups of related resources)
  - Excessive nesting (keep module hierarchy shallow)

### Code Quality

- Use comments to explain complex configurations and design decisions
- Write concise, efficient, and idiomatic configurations
- Avoid hard-coded values; use variables instead
- Avoid redundant comments; comments should add value and clarity

## Style and Formatting

- Follow Terraform best practices for resource naming and organization.
  - Use descriptive names for resources, variables, and outputs.
  - Use consistent naming conventions across all configurations.
- Follow the **Terraform Style Guide** for formatting.
  - Use consistent indentation (2 spaces for each level).
- Group related resources together in the same file.
  - Use a consistent naming convention for resource groups (e.g.,
    `providers.tf`, `variables.tf`, `network.tf`, `ecs.tf`, `mariadb.tf`).
- Place `depends_on` blocks at the very beginning of resource definitions to
  make dependency relationships clear.
  - Use `depends_on` only when necessary to avoid circular dependencies.
- Place `for_each` and `count` blocks at the beginning of resource definitions
  to clarify the resource's instantiation logic.
  - Use `for_each` for collections and `count` for numeric iterations.
  - Place them after `depends_on` blocks, if they are present.
- Place `lifecycle` blocks at the end of resource definitions.
- Alphabetize providers, variables, data sources, resources, and outputs within
  each file for easier navigation.
- Group related attributes together within blocks.
  - Place required attributes before optional ones, and comment each section
    accordingly.
  - Separate attribute sections with blank lines to improve readability.
  - Alphabetize attributes within each section for easier navigation.
- Use blank lines to separate logical sections of your configurations.
- Use `terraform fmt` to format your configurations automatically.
- Use `terraform validate` to check for syntax errors and ensure configurations
  are valid.
- Use `tflint` to check for style violations and ensure configurations follow
  best practices.

### Indentation and Spacing

- Use consistent indentation (2 spaces per level)
- Use blank lines to separate logical sections
- Separate attribute sections with blank lines for readability

### Variable and Output Documentation

- **Always include** `description` and `type` attributes for variables and
  outputs
- Use clear, concise descriptions explaining the purpose
- Use appropriate types: `string`, `number`, `bool`, `list`, `map`, `object`
  with sensible defaults

### Inline Comments

- Use comments to explain:
  - Purpose of resources and variables
  - Complex configurations or design decisions
  - Why specific approaches were chosen
- Avoid redundant comments; add value and clarity

### Module README

Include a `README.md` file in each module with the following structure:

````markdown
# Module Name

Short description of the module.

## How to use

```hcl
module "my_module" {
  source = "../modules/my_module"

  // Sample input variables
  var1 = "value1"
  var2 = "value2"
}
```

<!-- markdownlint-disable -->
<!-- BEGIN_TF_DOCS -->
<!-- END_TF_DOCS -->
````

### Documentation Generation

### Test File Structure

- Use `.tftest.hcl` extension for test files
- Write tests to validate functionality of configurations

### Test Coverage

- Cover both positive and negative scenarios
- Test expected success cases
- Test error handling and edge cases

### Test Properties

- Ensure tests are **idempotent** (can be run multiple times)
- Tests should have no side effects
- Each test should be independent of others - Check for style violations and
  best practices
  - Configuration:
    [`deploy/iac/iknite/.tflint.hcl`](../../deploy/iac/iknite/.tflint.hcl)
  - Run command: `terragrunt run --all -- validate`
  - Run regularly to catch issues earlyvalue2"

  }

  ```

  <!-- markdownlint-disable -->
  <!-- BEGIN_TF_DOCS -->
  <!-- END_TF_DOCS -->

  ```

- Include instructions for setting up and using the configurations.
- Use `terraform-docs` to generate documentation for your configurations
  automatically.

## Testing

- Write tests to validate the functionality of your Terraform configurations.
- Use the `.tftest.hcl` extension for test files.
- Write tests to cover both positive and negative scenarios.
- Ensure tests are idempotent and can be run multiple times without side
  effects.

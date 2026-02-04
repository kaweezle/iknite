<!-- cSpell: words tenv releaserepo testrepo kwzltfstate -->

# Infrastructure as Code (IaC)

This directory contains Terraform modules and Terragrunt configurations for
managing the Iknite project infrastructure.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Directory Structure](#directory-structure)
- [Getting Started](#getting-started)
- [Conventions and Best Practices](#conventions-and-best-practices)
- [Common Operations](#common-operations)
- [Troubleshooting](#troubleshooting)

## Overview

The Iknite infrastructure uses:

- **[Terraform](https://www.terraform.io/)** (v1.14.3+) - Infrastructure
  provisioning
- **[Terragrunt](https://terragrunt.gruntwork.io/)** (v0.97.2+) - DRY Terraform
  configuration management
- **[SOPS](https://github.com/mozilla/sops)** - Secrets encryption and
  management

All infrastructure is defined as code following the conventions documented in
[terraform.instructions.md](../../.github/instructions/terraform.instructions.md).

## Prerequisites

### Required Tools

#### 1. Install tenv (Terraform/Terragrunt Version Manager)

[tenv](https://github.com/tofuutils/tenv) allows you to manage multiple versions
of Terraform and Terragrunt. Install it using one of the following methods:

**On Alpine Linux:**

```bash
# Download latest version
TENV_LATEST_VERSION=$(curl --silent https://api.github.com/repos/tofuutils/tenv/releases/latest | jq -r .tag_name)
curl -O -L "https://github.com/tofuutils/tenv/releases/latest/download/tenv_${TENV_LATEST_VERSION}_amd64.deb"
sudo dpkg -i "tenv_${TENV_LATEST_VERSION}_amd64.deb"
```

**On macOS:**

```bash
brew install tenv
```

**On other Linux distributions:**

See the
[tenv installation guide](https://github.com/tofuutils/tenv#installation).

#### 2. Install Terraform and Terragrunt with tenv

```bash
# Install Terraform v1.14.3
tenv tf install 1.14.3
tenv tf use 1.14.3

# Install Terragrunt v0.97.2
tenv tg install 0.97.2
tenv tg use 0.97.2

# Verify installations
terraform --version
terragrunt --version
```

#### 3. Set Up Terraform Plugin Cache

To avoid downloading providers repeatedly, configure a shared plugin cache:

```bash
export TF_PLUGIN_CACHE_DIR="$HOME/.cache/terraform/plugin-cache"
mkdir -p "$TF_PLUGIN_CACHE_DIR"

# Add to your shell profile (~/.bashrc, ~/.zshrc, etc.)
echo 'export TF_PLUGIN_CACHE_DIR="$HOME/.cache/terraform/plugin-cache"' >> ~/.zshrc
```

#### 4. Install SOPS and Age

For managing encrypted secrets:

```bash
# Install SOPS
# On Alpine
apk add sops

# On macOS
brew install sops

# Install age for encryption
# On Alpine
apk add age

# On macOS
brew install age
```

#### 5. Configure SOPS/Age Keys

Secrets are encrypted using SOPS with age encryption. To decrypt secrets:

```bash
# Create age config directory
mkdir -p ~/.config/sops/age

# Add your age private key to ~/.config/sops/age/keys.txt
# Request the key from the project maintainers
```

## Directory Structure

```
deploy/iac/
├── README.md                    # This file
├── iknite/                      # Terragrunt units (infrastructure definitions)
│   ├── root.hcl                 # Root Terragrunt configuration
│   ├── secrets.sops.yaml        # Encrypted secrets (SOPS)
│   ├── .tflint.hcl             # TFLint configuration
│   ├── apkrepo/                # APK repository static site
│   ├── dns_iknite_app/         # DNS configuration for iknite.app
│   ├── iknite-image/           # OpenStack image management
│   ├── iknite-vm/              # VM deployment for testing
│   ├── releaserepo/            # Production APK repository
│   └── testrepo/               # Testing APK repository
└── modules/                     # Reusable Terraform modules
    ├── object-store-sync/      # Object storage synchronization
    ├── openstack-image/        # OpenStack image creation
    ├── openstack-vm/           # OpenStack VM provisioning
    └── public-object-store/    # Public object storage setup
```

### Terragrunt Units (`iknite/`)

Each subdirectory under `iknite/` represents a separate Terragrunt unit that:

- Has its own Terraform state file in S3 (`kwzltfstate` bucket)
- Can depend on other units via Terragrunt dependencies
- Includes the root configuration (`root.hcl`)
- May reference modules from the `modules/` directory

### Terraform Modules (`modules/`)

Reusable Terraform modules that encapsulate common infrastructure patterns.
Modules are referenced by Terragrunt units using repository-relative paths.

## Getting Started

### Initial Setup

1. **Clone the repository:**

   ```bash
   git clone https://github.com/kaweezle/iknite.git
   cd iknite/deploy/iac
   ```

2. **Verify prerequisites:**

   ```bash
   # Check Terraform
   terraform --version  # Should be v1.14.3+

   # Check Terragrunt
   terragrunt --version  # Should be v0.97.2+

   # Check SOPS
   sops --version

   # Verify age key is configured
   test -f ~/.config/sops/age/keys.txt && echo "Age key found" || echo "Age key missing"
   ```

3. **Configure credentials:**

   The project uses SOPS-encrypted secrets stored in `iknite/secrets.sops.yaml`.
   Ensure you have the decryption key configured (see step 4 in Prerequisites).

   Credentials include:
   - OVH OpenStack credentials
   - OVH API credentials
   - S3 credentials for state storage
   - Cloudflare credentials

### Working with Terragrunt Units

#### Navigate to a Unit

```bash
cd iknite/testrepo  # Example: test APK repository
```

#### Initialize the Unit

```bash
terragrunt init
```

This will:

- Generate `backend.tf` from `root.hcl`
- Generate `providers.tf` and `versions.tf` (if not present)
- Download required Terraform providers
- Initialize the S3 backend

#### Plan Changes

```bash
terragrunt plan
```

#### Apply Changes

```bash
terragrunt apply
```

#### View Dependency Graph

```bash
terragrunt graph
```

### Running Multiple Units

Terragrunt can run commands across all units:

```bash
# From the iknite/ directory
cd iknite/

# Initialize all units
terragrunt run-all init

# Plan all units
terragrunt run-all plan

# Apply all units (be careful!)
terragrunt run-all apply
```

## Conventions and Best Practices

For detailed conventions, see
[terraform.instructions.md](../../.github/instructions/terraform.instructions.md).

### Key Conventions

#### Resource Naming

- Prefer resource name `this` for single-instance resources
- Use descriptive names when multiple instances exist
- Use `for_each` over `count` for better resource management

#### Variable Design

- Use `map(object{...})` to group related values
- Map keys represent resource names
- Always include `description` and `type` for variables
- Mark sensitive variables with `sensitive = true`

#### Module Structure

Every module must have:

- `main.tf` - Resource definitions
- `variables.tf` - Input variables
- `outputs.tf` - Output values
- `versions.tf` - Required Terraform/provider versions
- `providers.tf` - Provider configurations
- `README.md` - Documentation (generated with `terraform-docs`)

#### State Management

- Remote state stored in OVH S3-compatible object storage
- Bucket: `kwzltfstate`
- State file path: `iknite/<unit-name>/terraform.tfstate`
- Access credentials configured in `root.hcl`

#### Security

- **Never commit secrets** to version control
- Use SOPS for all sensitive data
- Mark sensitive outputs with `sensitive = true`
- Use `.gitignore` to exclude:
  - `.terraform/` directories
  - `*.tfstate` files
  - `*.tfvars` files with credentials

## Common Operations

### Creating a New Terragrunt Unit

1. **Create the unit directory:**

   ```bash
   mkdir -p iknite/my-new-unit
   cd iknite/my-new-unit
   ```

2. **Create `terragrunt.hcl`:**

   ```hcl
   include "root" {
     path   = find_in_parent_folders("root.hcl")
     expose = true
   }

   terraform {
     source = "${get_repo_root()}/deploy/iac/modules/my-module"
   }

   inputs = {
     # Your inputs here
   }
   ```

3. **Initialize and plan:**

   ```bash
   terragrunt init
   terragrunt plan
   ```

### Creating a New Terraform Module

1. **Create the module directory:**

   ```bash
   mkdir -p modules/my-new-module
   cd modules/my-new-module
   ```

2. **Create required files:**

   ```bash
   touch main.tf variables.tf outputs.tf versions.tf providers.tf README.md
   ```

3. **Define the module** following the conventions in
   [terraform.instructions.md](../../.github/instructions/terraform.instructions.md).

4. **Generate documentation:**

   ```bash
   terraform-docs markdown table --output-file README.md .
   ```

### Adding a Dependency Between Units

In the dependent unit's `terragrunt.hcl`:

```hcl
dependency "other_unit" {
  config_path = "${get_parent_terragrunt_dir("root")}/other-unit"
}

inputs = {
  some_value = dependency.other_unit.outputs.some_output
}
```

### Updating Encrypted Secrets

```bash
# Edit secrets
sops iknite/secrets.sops.yaml

# The file will open in your editor
# Save and exit - SOPS will re-encrypt automatically
```

Or use the
[sops extension in VSCode](https://marketplace.visualstudio.com/items?itemName=signageos.signageos-vscode-sops)
for easier editing.

## Troubleshooting

### Common Issues

#### "No valid credential sources found"

**Problem:** Missing or invalid credentials.

**Solution:**

1. Verify age key exists: `test -f ~/.config/sops/age/keys.txt`
2. Try decrypting secrets manually: `sops -d iknite/secrets.sops.yaml`
3. Ensure `root.hcl` can access the secrets

#### "Error acquiring the state lock"

**Problem:** Another process has locked the state.

**Solution:**

```bash
# Force unlock (use with caution)
terragrunt force-unlock <LOCK_ID>
```

#### "Module not found"

**Problem:** Module path is incorrect.

**Solution:**

- Use absolute paths from repo root: `${get_repo_root()}/deploy/iac/modules/...`
- Run `terragrunt init` after changing module sources

#### "Provider version conflict"

**Problem:** Provider version mismatch between units.

**Solution:**

1. Check provider versions in `versions.tf`
2. Ensure consistency across all units
3. Run `terragrunt init -upgrade` to upgrade providers

### Getting Help

- **Terraform documentation:** https://www.terraform.io/docs
- **Terragrunt documentation:** https://terragrunt.gruntwork.io/docs
- **Project conventions:**
  [terraform.instructions.md](../../.github/instructions/terraform.instructions.md)
- **SOPS documentation:** https://github.com/mozilla/sops

## CI/CD Integration

The infrastructure is automatically managed through GitHub Actions. See
[.github/workflows/release.yml](../../.github/workflows/release.yml) for the
complete CI/CD pipeline.

Key workflows:

1. **APK Repository Upload** - Automatically syncs APK packages to object
   storage
2. **VM Image Testing** - Deploys and tests VM images on OpenStack
3. **DNS Updates** - Manages DNS records for iknite.app

## Additional Resources

- [Copilot Instructions](../../.github/copilot-instructions.md) - AI agent
  development guide
- [Project Structure](../../STRUCTURE.md) - Overall project architecture
- [Contributing Guide](../../CONTRIBUTING.md) - How to contribute
- [Build Instructions](../../BUILD.md) - Building the project

---

For questions or issues, please open an issue at
[github.com/kaweezle/iknite/issues](https://github.com/kaweezle/iknite/issues).

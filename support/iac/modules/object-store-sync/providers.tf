
provider "aws" {
  # No access keys needed as they come from OVH storage credentials

  skip_credentials_validation = true
  skip_region_validation      = true
  skip_requesting_account_id  = true
  skip_metadata_api_check     = true

  # s3_use_path_style          = true

  endpoints {
    s3 = "https://s3.${var.s3.region}.io.cloud.ovh.net/"
  }

  # Required dummy values
  region     = var.s3.region
  access_key = var.s3.access_key
  secret_key = var.s3.secret_key
}

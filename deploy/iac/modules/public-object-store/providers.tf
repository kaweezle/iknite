provider "openstack" {
  // Configured through the OS_CLOUD variable
  alias = "ovh"
}

provider "ovh" {
  endpoint           = var.ovh.endpoint
  application_key    = var.ovh.application_key
  application_secret = var.ovh.application_secret
  consumer_key       = var.ovh.consumer_key
}

provider "aws" {
  # No access keys needed as they come from OVH storage credentials

  skip_credentials_validation = true
  skip_region_validation      = true
  skip_requesting_account_id  = true
  skip_metadata_api_check     = true

  # s3_use_path_style          = true

  endpoints {
    s3 = "https://s3.${lower(var.region)}.io.cloud.ovh.net/"
  }

  # Required dummy values
  region     = lower(var.region)
  access_key = var.s3.access_key
  secret_key = var.s3.secret_key
}

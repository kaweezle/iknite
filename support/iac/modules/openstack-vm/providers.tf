provider "openstack" {
  auth_url            = var.openstack.auth_url
  user_domain_name    = var.openstack.user_domain_name
  project_domain_name = var.openstack.project_domain_name
  user_name           = var.openstack.user_name
  password            = var.openstack.password
  region              = var.openstack.region
  tenant_id           = var.openstack.tenant_id
}

provider "ovh" {
  endpoint           = var.ovh.endpoint
  application_key    = var.ovh.application_key
  application_secret = var.ovh.application_secret
  consumer_key       = var.ovh.consumer_key
}

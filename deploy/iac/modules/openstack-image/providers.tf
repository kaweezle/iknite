provider "openstack" {
  auth_url            = var.openstack.auth_url
  password            = var.openstack.password
  project_domain_name = var.openstack.project_domain_name
  region              = var.openstack.region
  tenant_id           = var.openstack.tenant_id
  user_domain_name    = var.openstack.user_domain_name
  user_name           = var.openstack.user_name
}

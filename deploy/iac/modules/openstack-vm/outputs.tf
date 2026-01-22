output "instances" {
  description = "Alpine instance IP address"
  value = { for k, v in openstack_compute_instance_v2.this : k => {
    name         = v.name
    access_ip_v4 = v.access_ip_v4
    access_ip_v6 = v.access_ip_v6
    metadata     = v.all_metadata
    tags         = v.all_tags
  } }
}

output "keypairs" {
  description = "Keypairs created"
  value = { for k, v in openstack_compute_keypair_v2.this : k => {
    name        = v.name
    public_key  = v.public_key
    fingerprint = v.fingerprint
  } }
}

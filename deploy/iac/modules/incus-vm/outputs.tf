output "instances" {
  value = { for k, v in incus_instance.this : k => {
    name        = v.name
    description = v.description
    project     = v.project
    remote      = v.remote
    ipv4        = v.ipv4_address
    ipv6        = v.ipv6_address
    status      = v.status
  } }
  description = "Incus instances created by this module, keyed by the instance key in the input variable. Each instance contains the following attributes: name, description, project, remote, ipv4, ipv6 and status."
}

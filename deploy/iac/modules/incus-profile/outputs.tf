output "profiles" {
  value = { for k, v in incus_profile.this : k => {
    name        = v.name
    description = v.description
    project     = v.project
    remote      = v.remote
  } }
  description = "Incus profiles"
}

output "buckets" {
  value = { for k, v in ovh_cloud_project_storage.this : k => {
    name         = v.name
    region       = v.region
    service_name = v.service_name
    virtual_host = v.virtual_host
    base_url     = "http://${v.name}.s3-website.${v.region}.io.cloud.ovh.net/"
  } }
  description = "The OVH Cloud Object Storage buckets created."
}

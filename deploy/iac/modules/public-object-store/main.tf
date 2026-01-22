# This module creates OVH Cloud Object Storage buckets and configures them for static website hosting.
# It also uploads static files to the buckets.
resource "ovh_cloud_project_storage" "this" {
  for_each = var.object_stores

  service_name = var.project_id
  region_name  = var.region
  name         = each.value.name
  limit        = each.value.limit

  versioning = each.value.versioning ? {
    status = "enabled"
  } : null
}

# This resource configures the public access block for the S3 bucket.
resource "aws_s3_bucket_acl" "this" {
  for_each = var.object_stores

  bucket = ovh_cloud_project_storage.this[each.key].name
  acl    = "public-read"
  # depends_on = [ aws_s3_bucket_public_access_block.this ]
}

# Make the bucket a static website
resource "aws_s3_bucket_website_configuration" "this" {
  for_each = var.object_stores

  bucket = ovh_cloud_project_storage.this[each.key].name

  index_document {
    suffix = each.value.index_document
  }

  error_document {
    key = each.value.error_document
  }
}

locals {
  # Filter the object stores to only include those with a static content path
  stores_with_static_content = { for k, v in var.object_stores : k => v if v.static_content_path != null }
}

# It uses the `hashicorp/dir/template` module to read files from the local filesystem and upload them to the bucket.
module "static_files" {
  for_each = local.stores_with_static_content
  source   = "hashicorp/dir/template"
  version  = ">=1.0.2,<2.0.0"

  base_dir = each.value.static_content_path
}

locals {
  # Flatten the list of files and create a map with bucket name and file key as the key
  files_map = { for item in flatten([
    for bucket_name, bucket in local.stores_with_static_content :
    [
      for file_key, file in module.static_files[bucket_name].files :
      {
        bucket_name = bucket_name
        bucket      = bucket
        file_key    = file_key
        file        = file
      }
    ]
  ]) : "${item.bucket_name}-${item.file_key}" => item }
}

# This resource uploads the files to the S3 bucket.
resource "aws_s3_object" "this" {
  for_each = local.files_map

  bucket       = each.value.bucket.name
  key          = each.value.file_key
  source       = each.value.file.source_path
  etag         = each.value.file.digests.md5
  content_type = each.value.file.content_type
  content      = each.value.file.content
  acl          = "public-read"
}

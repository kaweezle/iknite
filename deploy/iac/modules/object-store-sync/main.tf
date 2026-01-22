
# It uses the `hashicorp/dir/template` module to read files from the local filesystem and upload them to the bucket.
module "static_files" {
  for_each = var.object_stores
  source   = "hashicorp/dir/template"
  version  = ">=1.0.2,<2.0.0"

  base_dir = each.value.static_content_path
}

locals {
  # Flatten the list of files and create a map with bucket name and file key as the key
  files_map = { for item in flatten([
    for bucket_name, bucket in var.object_stores :
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
  key          = "${each.value.bucket.destination_prefix}${each.value.file_key}"
  source       = each.value.file.source_path
  etag         = each.value.file.digests.md5
  content_type = each.value.file.content_type
  content      = each.value.file.content
  acl          = "public-read"

  #   lifecycle {
  #     ignore_changes = [source]
  #   }
}

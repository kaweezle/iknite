output "files" {
  value = { for k, v in aws_s3_object.this : k => {
    bucket_name  = v.bucket
    key          = v.key
    etag         = v.etag
    content_type = v.content_type
  } }
  description = "The files uploaded to the S3 bucket."
}

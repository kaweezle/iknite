variable "github_token" {
  description = "GitHub personal access token with appropriate permissions"
  type        = string
  sensitive   = true
}

variable "github_owner" {
  description = "GitHub organization or user name that owns the resources"
  type        = string
  default     = null
}

variable "repositories" {
  description = "List of repository names (without owner prefix) to configure deploy keys and webhooks for"
  type        = list(string)
  default     = []
}

variable "organizations" {
  description = "List of organization names to configure webhooks for"
  type        = list(string)
  default     = []
}

variable "webhook_url" {
  description = "Webhook URL for ArgoCD to receive webhook events"
  type        = string
}

variable "webhook_secret" {
  description = "Secret for securing webhook requests"
  type        = string
  sensitive   = true
}

variable "deploy_key_public_key" {
  description = "SSH public key to be used as deploy key for git repositories"
  type        = string
}

variable "deploy_key_title" {
  description = "Title for the deploy key"
  type        = string
  default     = "ArgoCD Deploy Key"
}

variable "deploy_key_read_only" {
  description = "Whether the deploy key should be read-only"
  type        = bool
  default     = true
}

variable "webhook_events" {
  description = "List of events that should trigger the webhook"
  type        = list(string)
  default     = ["push", "pull_request"]
}

variable "webhook_active" {
  description = "Whether the webhook is active"
  type        = bool
  default     = true
}

variable "webhook_content_type" {
  description = "Content type for webhook payloads"
  type        = string
  default     = "json"
}

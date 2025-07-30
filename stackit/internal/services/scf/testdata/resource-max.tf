variable "project_id" {}
variable "name" {}
variable "platform_id" {}
variable "quota_id" {}
variable "suspended" {}

resource "stackit_scf_organization" "my-scf-org" {
  project_id  = var.project_id
  name        = var.name
  platform_id = var.platform_id
  quota_id    = var.quota_id
  suspended   = var.suspended
}

resource "stackit_scf_organization_manager" "my-scf-org-manager" {
  project_id = var.project_id
  org_id     = stackit_scf_organization.my-scf-org.org_id
}
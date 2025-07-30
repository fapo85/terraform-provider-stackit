variable "project_id" {}
variable "name" {}

resource "stackit_scf_organization" "my-scf-org" {
  project_id = var.project_id
  name       = var.name
}

resource "stackit_scf_organization_manager" "my-scf-org-manager" {
  project_id = var.project_id
  org_id     = stackit_scf_organization.my-scf-org.org_id
}
# How to Provisioning Cloud Foundry using Terrform

## Objective

This tutorial demonstrates how to provision Cloud Foundry resources by
integrating the STACKIT Terraform provider with the Cloud Foundry Terraform
provider. The STACKIT Terraform provider will create a managed Cloud Foundry
organization and set up a technical "org manager" user with
`organization_manager` permissions. These credentials, along with the Cloud
Foundry API URL (retrieved dynamically from a platform data resource), will then
be passed to the Cloud Foundry Terraform provider to manage resources within the
newly created organization.

### Output

The example configuration provisions a Cloud Foundry organization, mirroring the
structure created via the portal. It sets up three distinct spaces: `dev`, `qa`,
and `prod`. Furthermore, a specified user is assigned the `organization_manager`
and `organization_user` roles at the organization level, and the
`space_developer` role within each of the `dev`, `qa`, and `prod` spaces.

### Outside of the scope

This tutorial focuses specifically on the interaction between the STACKIT
Terraform provider and the Cloud Foundry Terraform provider. It assumes
familiarity with:

- Setting up a STACKIT project and configuring the STACKIT Terraform provider
  with a service account (details for which can be found in the general STACKIT
  documentation).
- Basic Terraform concepts, such as variables and locals.

Therefore, this document will not delve into these foundational topics, nor will
it cover every feature offered by the Cloud Foundry Terraform provider.

### Example configuration

```
terraform {
  required_providers {
    stackit = {
      source = "stackitcloud/stackit"
    }
    cloudfoundry = {
      source = "cloudfoundry/cloudfoundry"
    }
  }
}

variable "project_id" {
  type        = string
  description = "Id of the Project"
}

variable "org_name" {
  type        = string
  description = "Name of the Organization"
}

variable "admin_email" {
  type        = string
  description = "Users who are granted permissions"
}

provider "stackit" {
  default_region           = "eu01"
}

resource "stackit_scf_organization" "scf_org" {
  name       = var.org_name
  project_id = var.project_id
}

data "stackit_scf_platform" "scf_platform" {
  project_id = var.project_id
  guid       = stackit_scf_organization.scf_org.platform_id
}

resource "stackit_scf_organization_manager" "scf_manager" {
  project_id = var.project_id
  org_id     = stackit_scf_organization.scf_org.org_id
}

provider "cloudfoundry" {
  api_url  = data.stackit_scf_platform.scf_platform.api_url
  user     = stackit_scf_organization_manager.scf_manager.username
  password = stackit_scf_organization_manager.scf_manager.password
}

locals {
  spaces    = ["dev", "qa", "prod"]
}

resource "cloudfoundry_org_role" "org_user" {
  username = var.admin_email
  type     = "organization_user"
  org      = stackit_scf_organization.scf_org.org_id
}

resource "cloudfoundry_org_role" "org_manager" {
  username = var.admin_email
  type     = "organization_manager"
  org      = stackit_scf_organization.scf_org.org_id
}

resource "cloudfoundry_space" "spaces" {
  for_each = toset(local.spaces)
  name     = each.key
  org      = stackit_scf_organization.scf_org.org_id
}

resource "cloudfoundry_space_role" "space_developer" {
  for_each = toset(local.spaces)
  username = var.admin_email
  type     = "space_developer"
  depends_on = [ cloudfoundry_org_role.org_user ]
  space     = cloudfoundry_space.spaces[each.key].id
}
```

## Taken apart

### STACKIT Provider Configuration

```
provider "stackit" {
  default_region           = "eu01"
}
```

The SCF API is regionalized to ensure fault isolation, meaning each region
operates independently. For this reason, we set `default_region` in the provider
configuration, which applies to all resources unless explicitly overridden.
Additionally, appropriate access data for the corresponding STACKIT project must
be provided for the provider to function.

See:
[STACKIT Terraform Provider Documentation](https://registry.terraform.io/providers/stackitcloud/stackit/latest/docs)

### stackit_scf_organization.scf_org

```
resource "stackit_scf_organization" "scf_org" {
  name       = var.org_name
  project_id = var.project_id
}
```

This resource provisions the Cloud Foundry organization, which serves as the
foundational container within the Cloud Foundry environment. Each Cloud Foundry
provider configuration is scoped to a specific organization. The organization's
name, defined by a variable, must be unique across the platform. This
organization is created within a designated STACKIT project, requiring the
STACKIT provider to be configured with the necessary permissions for that
project.

### stackit_scf_organization_manager.scf_manager

```
resource "stackit_scf_organization_manager" "scf_manager" {
  project_id = var.project_id
  org_id     = stackit_scf_organization.scf_org.org_id
}
```

This resource creates a technical user within the Cloud Foundry Organization,
designated as an "organization manager." This user is intrinsically linked to
the organization and will be automatically deleted when the organization is
removed. The user is assigned the `organization_manager` permission within the
Cloud Foundry organization.

### stackit_scf_platform.scf_platform

```
data "stackit_scf_platform" "scf_platform" {
  project_id = var.project_id
  guid       = stackit_scf_organization.scf_org.platform_id
}
```

This data source retrieves properties of the Cloud Foundry platform where the
organization is provisioned. It does not create any resources within the
project, but rather provides information about an existing platform.

### Cloud Foundry Provider Configuration

```
provider "cloudfoundry" {
  api_url  = data.stackit_scf_platform.scf_platform.api_url
  user     = stackit_scf_organization_manager.scf_manager.username
  password = stackit_scf_organization_manager.scf_manager.password
}
```

The Cloud Foundry provider is configured to manage resources within the newly
created organization. It uses the API URL retrieved from the
`stackit_scf_platform` data source to communicate with the Cloud Foundry
platform and authenticates using the credentials of the
`stackit_scf_organization_manager` technical user.

See:
[Cloud Foundry Terraform Provider Documentation](https://registry.terraform.io/providers/cloudfoundry/cloudfoundry/latest/docs)

## Deployment

### init

```
terraform init
```

The `terraform init` command initializes the working directory and downloads the
necessary provider plugins.

### Create Initial Resources

```
terraform apply -target stackit_scf_organization_manager.scf_manager
```

This initial `terraform apply` command provisions the resources required to
initialize the Cloud Foundry Terraform provider. This step is only required for
the initial setup; subsequent `terraform apply` commands that modify resources
within the Cloud Foundry organization do not require the `-target` flag.

### Apply Complete Configuration

```
terraform apply
```

This command applies the complete Terraform configuration, provisioning all
defined resources within the Cloud Foundry organization.

## Verification

To verify the deployment, use the `cf apps`, `cf services`, and `cf routes`
commands.

see:

[Cloud Foundry Documentation](https://docs.cloudfoundry.org/)

[Cloud Foundry CLI Reference Guide](https://cli.cloudfoundry.org/)

---
page_title: "Using the provider with Pulumi"
subcategory: ""
description: |-
  Use this provider from a Pulumi program in TypeScript, Python, Go, .NET or Java, with no separate package to install.
---

# Using the provider with Pulumi

Pulumi can use any provider listed in the OpenTofu registry, and this one is
listed there. There is no separate Pulumi package: one command generates a typed
SDK inside your project.

## Add the provider

From the directory holding `Pulumi.yaml`:

```sh
pulumi package add terraform-provider uttej-badwane/anthropic-enterprise
```

To pin a version, add it after the name:

```sh
pulumi package add terraform-provider uttej-badwane/anthropic-enterprise 0.10.1
```

The version is recorded in `Pulumi.yaml`. To upgrade later, change it there and
run `pulumi install`.

The generated package is named after the provider:

| Language | Import |
|---|---|
| TypeScript | `import * as anthropicEnterprise from "@pulumi/anthropic-enterprise";` |
| Python | `import pulumi_anthropic_enterprise as anthropic` |

## Credentials

Provider settings live under the `anthropic-enterprise:` namespace and use
**camelCase**. Pulumi ignores a snake_case key such as `admin_api_key` without
an error, which is easy to miss.

```sh
pulumi config set --secret anthropic-enterprise:adminApiKey sk-ant-admin01-...
```

| Terraform attribute | Pulumi config key |
|---|---|
| `admin_api_key` | `anthropic-enterprise:adminApiKey` |
| `oauth_token` | `anthropic-enterprise:oauthToken` |
| `enterprise_api_key` | `anthropic-enterprise:enterpriseApiKey` |
| `compliance_api_key` | `anthropic-enterprise:complianceApiKey` |
| `analytics_api_key` | `anthropic-enterprise:analyticsApiKey` |
| `api_key` | `anthropic-enterprise:apiKey` |
| `workspace_id` | `anthropic-enterprise:workspaceId` |

Set secrets with `--secret` so they are encrypted in the stack configuration.
Which key each resource needs is the same as with Terraform; see
[Choosing credentials](./credentials). The `ANTHROPIC_*` environment variables
work too.

## Write the program

Resource types drop the `anthropic_` prefix, and data sources become functions.

```typescript
import * as anthropicEnterprise from "@pulumi/anthropic-enterprise";

const production = new anthropicEnterprise.Workspace("production", {
    name: "Production",
});

const org = anthropicEnterprise.getOrganizationOutput({});

export const workspaceId = production.id;
export const organizationName = org.name;
```

```python
import pulumi
import pulumi_anthropic_enterprise as anthropic

production = anthropic.Workspace("production", name="Production")
org = anthropic.get_organization()

pulumi.export("workspace_id", production.id)
```

| Terraform | Pulumi (TypeScript) |
|---|---|
| `anthropic_workspace` | `Workspace` |
| `anthropic_rbac_group` | `RbacGroup` |
| `anthropic_api_key` | `ApiKey` |
| `data.anthropic_workspaces` | `getWorkspaces()` / `getWorkspacesOutput()` |

Every resource and data source is available. Behaviour is the provider's own,
because Pulumi runs this provider's binary: a rename of a workspace is an update
in place, and destroy archives or deletes exactly as the resource's
documentation describes.

## What does not carry over

**`anthropic_federation_token` is not available.** It is an ephemeral resource,
and Pulumi's bridge does not generate ephemeral resources, so the SDK has no
equivalent. The [workload identity federation guide](./ci-cd) applies to
Terraform and OpenTofu only; a Pulumi pipeline needs to obtain its credential
another way.

**Write-only attributes have not been exercised through Pulumi.** The
resources that carry them, `VaultCredential` and `Deployment`, are generated, but
applying a secret through them has not been tested. Check the plan before
relying on it.

## Moving from Terraform

The resource ids are the same, so an object Terraform manages can be adopted by
a Pulumi program with `pulumi import`, for example:

```sh
pulumi import anthropic-enterprise:index/workspace:Workspace production wrkspc_01ExampleWorkspaceId00000
```

Remove it from Terraform state first with `terraform state rm`, so two tools do
not both believe they own it.

An imported object is guarded twice. Pulumi marks every imported resource
protected, so `pulumi destroy` refuses to remove it until you unprotect it, and
this provider imports a workspace with `archive_on_destroy = false`, so even an
unprotected destroy only drops it from the stack instead of archiving a real
workspace. Opt in to both before destroy can reach the organization.

#!/usr/bin/env bash
# tfplugindocs writes an empty subcategory into every generated page, which the
# Terraform Registry renders as one flat list of all ninety resources and data
# sources. Group the pages instead, so the registry sidebar has sections.
#
# Runs after tfplugindocs; `make generate` invokes it. A page whose name is not
# mapped is an error rather than a silent fallback, so new resources cannot land
# back in the flat list unnoticed.
set -euo pipefail

cd "$(dirname "$0")/.."

subcategory_for() {
  case "$1" in
    compliance_*)
      echo "Compliance" ;;
    analytics_* | usage_report | cost_report | claude_code_usage_report)
      echo "Usage and Analytics" ;;
    rbac_* | spend_limit*)
      echo "Claude Enterprise" ;;
    skill | skills | skill_versions)
      echo "Skills" ;;
    agent | agents | agent_versions | environment | environments | vault | vaults | \
    vault_credential | vault_credentials | deployment | deployments | deployment_run | deployment_runs | \
    memory_store | memory_stores)
      echo "Managed Agents" ;;
    service_account | service_accounts | workspace_service_account | workspace_service_accounts | federation_*)
      echo "Service Accounts and Federation" ;;
    workspace | workspaces | workspace_member | workspace_members | workspace_rate_limits | \
    external_key | external_keys)
      echo "Workspaces" ;;
    organization | user | users | invite | invites | api_key | api_keys | rate_limits)
      echo "Console Organization" ;;
    model | models)
      echo "Models" ;;
    *)
      return 1 ;;
  esac
}

status=0
# Ephemeral resources render into their own directory, so they are globbed
# alongside the others rather than being silently left ungrouped.
shopt -s nullglob
for file in docs/resources/*.md docs/data-sources/*.md docs/ephemeral-resources/*.md; do
  name="$(basename "$file" .md)"
  if ! subcategory="$(subcategory_for "$name")"; then
    echo "set-subcategories: no subcategory mapped for $file" >&2
    status=1
    continue
  fi
  # sed -i differs between GNU and BSD, so write through a temporary file.
  sed "s|^subcategory: \"\"$|subcategory: \"$subcategory\"|" "$file" >"$file.tmp"
  mv "$file.tmp" "$file"
done

if [[ $status -ne 0 ]]; then
  echo "set-subcategories: add the page above to subcategory_for in $0" >&2
fi
exit $status

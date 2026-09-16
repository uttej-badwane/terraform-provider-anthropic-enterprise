data "anthropic_workspaces" "all" {
  include_archived = false
}

output "workspace_names" {
  value = data.anthropic_workspaces.all.workspaces[*].name
}

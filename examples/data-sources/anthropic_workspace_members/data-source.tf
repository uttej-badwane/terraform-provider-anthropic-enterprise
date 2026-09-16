data "anthropic_workspace_members" "production" {
  workspace_id = "wrkspc_01ExampleWorkspaceId000000"
}

output "production_member_ids" {
  value = data.anthropic_workspace_members.production.members[*].user_id
}

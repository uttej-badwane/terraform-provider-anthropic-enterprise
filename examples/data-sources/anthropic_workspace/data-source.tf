data "anthropic_workspace" "by_id" {
  id = "wrkspc_01ExampleWorkspaceId000000"
}

data "anthropic_workspace" "by_name" {
  name = "Production"
}

data "anthropic_service_account" "by_name" {
  name = "inference-worker"
}

data "anthropic_service_account" "by_id" {
  id = "svac_01ExampleServiceAcct000000"
}

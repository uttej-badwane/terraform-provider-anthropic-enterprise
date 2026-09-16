data "anthropic_deployment" "nightly" {
  id = "depl_01ExampleDeployId00000000"
}

output "next_runs" {
  value = data.anthropic_deployment.nightly.upcoming_runs_at
}

data "anthropic_deployment_run" "last" {
  id = "drun_01ExampleRunId00000000000"
}

# The scheduled slot the run belongs to, which can differ from created_at when
# a firing is delayed.
output "scheduled_for" {
  value = data.anthropic_deployment_run.last.trigger_scheduled_at
}

# error_json is null on a successful run, so guard before decoding it.
output "failure_message" {
  value = try(jsondecode(data.anthropic_deployment_run.last.error_json).message, null)
}

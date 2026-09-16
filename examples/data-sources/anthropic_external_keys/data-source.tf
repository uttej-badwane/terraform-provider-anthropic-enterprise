data "anthropic_external_keys" "all" {}

output "attached_keys" {
  value = [for k in data.anthropic_external_keys.all.external_keys : k.id if k.attachment_type == "attached"]
}

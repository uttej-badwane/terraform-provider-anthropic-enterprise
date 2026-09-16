data "anthropic_memory_stores" "all" {}

output "memory_store_names" {
  value = data.anthropic_memory_stores.all.memory_stores[*].name
}

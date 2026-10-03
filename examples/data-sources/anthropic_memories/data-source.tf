data "anthropic_memories" "style" {
  memory_store_id = anthropic_memory_store.conventions.id
  path_prefix     = "/style/"
}

output "style_paths" {
  value = [for m in data.anthropic_memories.style.memories : m.path]
}

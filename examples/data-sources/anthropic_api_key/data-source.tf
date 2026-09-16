data "anthropic_api_key" "example" {
  id = "apikey_01ExampleApiKeyId00000000"
}

output "api_key_status" {
  value = data.anthropic_api_key.example.status
}

## v0.1.0 (2026-09-15)

FEATURES:

* **New Provider:** `uttej-badwane/anthropic-enterprise` (resource prefix `anthropic_`, declared with local name `anthropic`) with credential classes (`admin_api_key`, `oauth_token`, `enterprise_api_key`) and an in-process mock of the Admin API for acceptance tests
* **New Resource:** `anthropic_workspace`
* **New Resource:** `anthropic_workspace_member`
* **New Resource:** `anthropic_api_key` (import-only; manages name and status)
* **New Resource:** `anthropic_invite`
* **New Resource:** `anthropic_user` (import-only; manages role)
* **New Resource:** `anthropic_service_account`
* **New Resource:** `anthropic_workspace_service_account`
* **New Resource:** `anthropic_federation_issuer`
* **New Resource:** `anthropic_federation_rule`
* **New Resource:** `anthropic_federation_rule_workspace`
* **New Resource:** `anthropic_external_key`
* **New Resource:** `anthropic_rbac_group`
* **New Resource:** `anthropic_rbac_group_member`
* **New Resource:** `anthropic_spend_limit`
* **New Resource:** `anthropic_compliance_settings` (singleton toggle for the Compliance API)
* **New Data Source:** `anthropic_organization`, `anthropic_user`, `anthropic_users`, `anthropic_invite`, `anthropic_invites`, `anthropic_workspace`, `anthropic_workspaces`, `anthropic_workspace_member`, `anthropic_workspace_members`, `anthropic_api_key`, `anthropic_api_keys`, `anthropic_rate_limits`, `anthropic_workspace_rate_limits`, `anthropic_external_key`, `anthropic_external_keys`, `anthropic_compliance_settings`
* **New Data Source:** `anthropic_service_account`, `anthropic_service_accounts`, `anthropic_workspace_service_accounts`, `anthropic_federation_issuer`, `anthropic_federation_issuers`, `anthropic_federation_rule`, `anthropic_federation_rules`, `anthropic_federation_rule_workspaces`
* **New Data Source:** `anthropic_rbac_group`, `anthropic_rbac_groups`, `anthropic_rbac_group_members`, `anthropic_rbac_role`, `anthropic_rbac_roles`, `anthropic_spend_limit`, `anthropic_spend_limits`, `anthropic_spend_limit_increase_request`, `anthropic_spend_limit_increase_requests`
* **New Data Source:** `anthropic_usage_report`, `anthropic_cost_report`, `anthropic_claude_code_usage_report`, `anthropic_analytics_summaries`
* **New Data Source:** `anthropic_compliance_organizations`, `anthropic_compliance_organization_users`, `anthropic_compliance_roles`, `anthropic_compliance_role`, `anthropic_compliance_groups`, `anthropic_compliance_group_members`, `anthropic_compliance_effective_settings`
* Provider attributes `compliance_api_key` and `analytics_api_key` (both fall back to `enterprise_api_key`)
* **New Data Source:** `anthropic_analytics_users`, `anthropic_analytics_skills`, `anthropic_analytics_connectors`, `anthropic_analytics_plugins`, `anthropic_analytics_artifacts`, `anthropic_analytics_usage_report`, `anthropic_analytics_cost_report`, `anthropic_analytics_user_usage_report`, `anthropic_analytics_user_cost_report`
* Provider attributes `api_key` and `workspace_id` for the Managed Agents control plane and Skills API (beta)
* **New Resource:** `anthropic_agent`, `anthropic_environment`, `anthropic_vault`, `anthropic_vault_credential`, `anthropic_deployment`, `anthropic_memory_store`, `anthropic_skill`
* **New Data Source:** `anthropic_agent`, `anthropic_agents`, `anthropic_agent_versions`, `anthropic_environment`, `anthropic_environments`, `anthropic_vault`, `anthropic_vaults`, `anthropic_vault_credentials`, `anthropic_deployment`, `anthropic_deployments`, `anthropic_memory_store`, `anthropic_memory_stores`, `anthropic_skill`, `anthropic_skills`, `anthropic_skill_versions`

NOTES:

* Workspaces, service accounts, federation issuers and federation rules cannot be deleted through the Admin API; destroy archives them unless `archive_on_destroy = false`.
* Workspace membership reads use the list endpoint because the per-member GET is served from a cache that updates do not invalidate for tens of seconds; membership writes retry while a fresh membership is not yet visible and wait for the new role to read back.
* `anthropic_federation_rule` re-reads the rule after create and update because the API's write responses omit the read-time fields `issuer_name` and `service_account_name`.
* Verified against a live Console organization: organization, users, workspaces, API keys and rate-limit data sources, the `anthropic_workspace`, `anthropic_workspace_member`, `anthropic_service_account`, `anthropic_workspace_service_account`, `anthropic_federation_issuer` and `anthropic_federation_rule` lifecycles, and the full Managed Agents graph (environment, memory store, vault, vault credential, skill, agent, deployment) including an empty re-plan and imports.
* Enterprise (`rbac_*`, `spend_limit`, compliance, analytics) resources and `anthropic_invite` are verified against the mock only until those credentials and a disposable mailbox are available.
* Vault credential secrets use write-only attributes and require Terraform 1.11 or newer.
* `anthropic_analytics_chat_projects` is not included: the endpoint's reference page was unavailable when the schema was captured.

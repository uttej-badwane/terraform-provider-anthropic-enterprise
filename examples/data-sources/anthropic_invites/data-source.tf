data "anthropic_invites" "pending" {
  statuses = ["pending"]
}

output "pending_invite_emails" {
  value = data.anthropic_invites.pending.invites[*].email
}

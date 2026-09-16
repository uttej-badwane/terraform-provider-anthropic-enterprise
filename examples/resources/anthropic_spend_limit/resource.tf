# Cap a user at 500.00 per month (amount is in cents).
resource "anthropic_spend_limit" "alice_monthly" {
  user_id = "user_01ExampleUserId0000000000"
  period  = "monthly"
  amount  = "50000"
}

# Explicitly remove the daily cap for the same user.
resource "anthropic_spend_limit" "alice_daily_uncapped" {
  user_id = "user_01ExampleUserId0000000000"
  period  = "daily"
}

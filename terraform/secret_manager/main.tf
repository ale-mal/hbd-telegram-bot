resource "aws_secretsmanager_secret" "bot_token" {
  name                    = "BotToken"
  description             = "The token of the bot"
}

resource "aws_secretsmanager_secret_version" "bot_token" {
  secret_id     = aws_secretsmanager_secret.bot_token.id
  secret_string = var.bot_token
}

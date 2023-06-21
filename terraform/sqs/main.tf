resource "aws_sqs_queue" "message_queue" {
  name                       = var.queue_name
  delay_seconds              = 0
  max_message_size           = 4096
  message_retention_seconds  = 345600
  visibility_timeout_seconds = 60

  redrive_policy = var.dead_letter_queue_arn != "" ? jsonencode({
    deadLetterTargetArn = var.dead_letter_queue_arn
    maxReceiveCount     = 5
  }) : null
}

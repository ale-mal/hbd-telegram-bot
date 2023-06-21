output "sqs_arn" {
    value = aws_sqs_queue.message_queue.arn
}

output "sqs_name" {
    value = aws_sqs_queue.message_queue.name
}
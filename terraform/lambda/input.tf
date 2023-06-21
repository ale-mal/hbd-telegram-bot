variable "lambda_name" {
  description = "The name of the Lambda function"
  type        = string
}

variable "sqs_arn" {
  description = "The ARN of the SQS queue"
  type        = string
}

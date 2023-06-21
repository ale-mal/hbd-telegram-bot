variable "api_name" {
    description = "The name of the API Gateway"
    type        = string
}

variable "sqs_name" {
    description = "The name of the SQS queue"
    type        = string
}

variable "sqs_arn" {
    description = "The ARN of the SQS queue"
    type        = string
}
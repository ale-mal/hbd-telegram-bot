variable "lambda_name" {
  description = "The name of the Lambda function"
  type        = string
}

variable "stream_arn" {
  description = "The ARN of the Kinesis stream"
  type        = string
}

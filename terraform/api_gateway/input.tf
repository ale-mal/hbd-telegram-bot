variable "api_name" {
  description = "The name of the API Gateway"
  type        = string
}

variable "stream_name" {
  description = "The name of the Kinesis stream"
  type        = string
}

variable "stream_arn" {
  description = "The ARN of the Kinesis stream"
  type        = string
}

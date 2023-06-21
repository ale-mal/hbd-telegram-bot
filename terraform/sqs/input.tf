variable "queue_name" {
  description = "The name of the SQS queue"
  type        = string
}

variable "dead_letter_queue_arn" {
  description = "The ARN of the dead letter queue"
  type        = string
  default     = ""
}

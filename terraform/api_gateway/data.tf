data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

data "aws_iam_policy_document" "assume_api_gateway_role" {
    statement {
        actions = ["sts:AssumeRole"]

        principals {
            type        = "Service"
            identifiers = ["apigateway.amazonaws.com"]
        }
    }
}

data "aws_iam_policy_document" "api_gateway_policy" {
  statement {
    actions = [
      "kinesis:PutRecord",
    ]
    effect    = "Allow"
    resources = [var.stream_arn]
  }
}

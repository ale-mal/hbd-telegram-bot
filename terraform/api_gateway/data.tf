data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

data "aws_iam_policy_document" "api_gateway_policy" {
  statement {
    actions = [
      "sqs:SendMessage",
    ]
    effect    = "Allow"
    resources = [var.sqs_arn]
  }
}

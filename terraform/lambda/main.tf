resource "aws_iam_policy" "lambda_policy" {
  name   = "LambdaPolicy"
  path   = "/"
  policy = data.aws_iam_policy_document.lambda_policy.json
}

resource "aws_iam_role" "lambda_role" {
  name                = "LambdaRole"
  assume_role_policy  = data.aws_iam_policy_document.assume_lambda_role.json
  managed_policy_arns = [aws_iam_policy.lambda_policy.arn]
}

resource "aws_lambda_function" "lambda" {
  function_name = var.lambda_name
  handler       = local.binary_name
  role          = aws_iam_role.lambda_role.arn
  memory_size   = 128

  filename         = local.archive_path
  source_code_hash = data.archive_file.archive.output_base64sha256

  runtime = "go1.x"

  layers = [local.layer_arn]
}

resource "aws_lambda_event_source_mapping" "event_mapping" {
  event_source_arn  = var.stream_arn
  function_name     = aws_lambda_function.lambda.function_name
  enabled           = true
  starting_position = "LATEST"
}

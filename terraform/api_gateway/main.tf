resource "aws_iam_policy" "api_gateway_policy" {
  name   = "ApiGatewayPolicy"
  path   = "/"
  policy = data.aws_iam_policy_document.api_gateway_policy.json
}

resource "aws_iam_role" "api_gateway_role" {
  name = "APIGatewayRole"
  assume_role_policy = data.aws_iam_policy_document.assume_api_gateway_role.json
  managed_policy_arns = [aws_iam_policy.api_gateway_policy.arn]
}

resource "aws_api_gateway_rest_api" "api" {
  name = var.api_name
}

resource "aws_api_gateway_resource" "api_resource" {
  rest_api_id = aws_api_gateway_rest_api.api.id
  parent_id   = aws_api_gateway_rest_api.api.root_resource_id
  path_part   = "bot"
}

resource "aws_api_gateway_method" "api_method" {
  rest_api_id   = aws_api_gateway_rest_api.api.id
  resource_id   = aws_api_gateway_resource.api_resource.id
  http_method   = "POST"
  authorization = "NONE"
}

resource "aws_api_gateway_integration" "api_integration" {
  rest_api_id             = aws_api_gateway_rest_api.api.id
  resource_id             = aws_api_gateway_resource.api_resource.id
  http_method             = aws_api_gateway_method.api_method.http_method
  integration_http_method = "POST"
  type                    = "AWS"
  uri                     = "arn:aws:apigateway:${data.aws_region.current.name}:kinesis:action/PutRecord"
  credentials             = aws_iam_role.api_gateway_role.arn
  passthrough_behavior    = "NEVER"

  request_templates = {
    "application/json" = <<EOF
{
  "StreamName": "${var.stream_name}",
  "PartitionKey": "$input.path('$.update_id')",
  "Data": "$util.base64Encode($input.body)"
}
EOF
  }
}

resource "aws_api_gateway_method_response" "api_method_response" {
  rest_api_id = aws_api_gateway_rest_api.api.id
  resource_id = aws_api_gateway_resource.api_resource.id
  http_method = aws_api_gateway_method.api_method.http_method
  status_code = "200"

  response_models = {
    "application/json" = "Empty"
  }
}

resource "aws_api_gateway_integration_response" "api_integration_response" {
  rest_api_id = aws_api_gateway_rest_api.api.id
  resource_id = aws_api_gateway_resource.api_resource.id
  http_method = aws_api_gateway_method.api_method.http_method
  status_code = aws_api_gateway_method_response.api_method_response.status_code

  depends_on = [aws_api_gateway_integration.api_integration]
}

resource "aws_api_gateway_deployment" "api_deployment" {
  depends_on  = [aws_api_gateway_integration.api_integration]
  rest_api_id = aws_api_gateway_rest_api.api.id
  stage_name  = "prod"

  variables = {
    deployed_at = timestamp()
  }
}

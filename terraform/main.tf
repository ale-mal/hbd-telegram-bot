module "kinesis" {
  source      = "./kinesis"
  stream_name = "message_stream"
}

module "api_gateway" {
  source      = "./api_gateway"
  api_name    = "message_api"
  stream_arn  = module.kinesis.stream_arn
  stream_name = module.kinesis.stream_name
}

module "lambda" {
  source      = "./lambda"
  lambda_name = "message_lambda"
  stream_arn  = module.kinesis.stream_arn
}

module "secret_manager" {
  source    = "./secret_manager"
  bot_token = var.bot_token
}

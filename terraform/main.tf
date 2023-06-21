module "dead_message_queue" {
  source     = "./sqs"
  queue_name = "dead_message_queue"
}

module "message_queue" {
  source                = "./sqs"
  queue_name            = "message_queue"
  dead_letter_queue_arn = module.dead_message_queue.sqs_arn
}

module "api_gateway" {
  source   = "./api_gateway"
  api_name = "message_api"
  sqs_name = module.message_queue.sqs_name
  sqs_arn  = module.message_queue.sqs_arn
}

module "lambda" {
  source       = "./lambda"
  lambda_name  = "message_lambda"
  sqs_arn      = module.message_queue.sqs_arn
}

module "secret_manager" {
  source    = "./secret_manager"
  bot_token = var.bot_token
}

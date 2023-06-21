module "message_queue" {
  source     = "./sqs"
  queue_name = "message_queue"
}

module "api_gateway" {
  source   = "./api_gateway"
  api_name = "message_api"
  sqs_name = module.message_queue.sqs_name
  sqs_arn  = module.message_queue.sqs_arn
}

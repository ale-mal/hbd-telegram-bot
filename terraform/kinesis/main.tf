resource "aws_kinesis_stream" "stream" {
  name             = var.stream_name
  retention_period = 24

  stream_mode_details {
    stream_mode = "ON_DEMAND"
  }
}
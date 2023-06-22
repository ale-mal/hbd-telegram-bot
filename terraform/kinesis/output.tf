output "stream_arn" {
    value = aws_kinesis_stream.stream.arn
}

output "stream_name" {
    value = aws_kinesis_stream.stream.name
}

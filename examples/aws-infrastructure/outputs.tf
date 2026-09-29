output "alb_dns" {
  value = aws_lb.app.dns_name
}

output "instance_id" {
  value = aws_instance.app.id
}

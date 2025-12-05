# EAMSA 512 AWS Deployment with Terraform

terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

# Variables
variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Environment name"
  type        = string
  default     = "production"
}

variable "instance_type" {
  description = "EC2 instance type"
  type        = string
  default     = "t3.medium"
}

variable "desired_capacity" {
  description = "Desired number of instances"
  type        = number
  default     = 3
}

# VPC
resource "aws_vpc" "eamsa512" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name = "eamsa512-vpc"
  }
}

# Public Subnets
resource "aws_subnet" "public" {
  count             = 2
  vpc_id            = aws_vpc.eamsa512.id
  cidr_block        = "10.0.${count.index + 1}.0/24"
  availability_zone = data.aws_availability_zones.available.names[count.index]

  map_public_ip_on_launch = true

  tags = {
    Name = "eamsa512-public-${count.index + 1}"
  }
}

# Private Subnets
resource "aws_subnet" "private" {
  count             = 2
  vpc_id            = aws_vpc.eamsa512.id
  cidr_block        = "10.0.${count.index + 10}.0/24"
  availability_zone = data.aws_availability_zones.available.names[count.index]

  tags = {
    Name = "eamsa512-private-${count.index + 1}"
  }
}

# Internet Gateway
resource "aws_internet_gateway" "eamsa512" {
  vpc_id = aws_vpc.eamsa512.id

  tags = {
    Name = "eamsa512-igw"
  }
}

# Route Table
resource "aws_route_table" "public" {
  vpc_id = aws_vpc.eamsa512.id

  route {
    cidr_block      = "0.0.0.0/0"
    gateway_id      = aws_internet_gateway.eamsa512.id
  }

  tags = {
    Name = "eamsa512-public-rt"
  }
}

resource "aws_route_table_association" "public" {
  count          = 2
  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

# Security Groups
resource "aws_security_group" "alb" {
  name        = "eamsa512-alb"
  description = "ALB security group"
  vpc_id      = aws_vpc.eamsa512.id

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "eamsa512-alb-sg"
  }
}

resource "aws_security_group" "instance" {
  name        = "eamsa512-instance"
  description = "Instance security group"
  vpc_id      = aws_vpc.eamsa512.id

  ingress {
    from_port       = 8080
    to_port         = 8080
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
  }

  ingress {
    from_port       = 9090
    to_port         = 9090
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "eamsa512-instance-sg"
  }
}

# ALB
resource "aws_lb" "eamsa512" {
  name               = "eamsa512-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = aws_subnet.public[*].id

  tags = {
    Name = "eamsa512-alb"
  }
}

resource "aws_lb_target_group" "eamsa512" {
  name        = "eamsa512-tg"
  port        = 8080
  protocol    = "HTTPS"
  vpc_id      = aws_vpc.eamsa512.id
  target_type = "instance"

  health_check {
    healthy_threshold   = 2
    unhealthy_threshold = 2
    timeout             = 5
    interval            = 30
    path                = "/api/v1/health"
    matcher             = "200"
    port                = "8080"
  }

  tags = {
    Name = "eamsa512-tg"
  }
}

resource "aws_lb_listener" "https" {
  load_balancer_arn = aws_lb.eamsa512.arn
  port              = 443
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS-1-2-2017-01"
  certificate_arn   = aws_acm_certificate.eamsa512.arn

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.eamsa512.arn
  }
}

resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.eamsa512.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type = "redirect"

    redirect {
      port        = "443"
      protocol    = "HTTPS"
      status_code = "HTTP_301"
    }
  }
}

# ACM Certificate
resource "aws_acm_certificate" "eamsa512" {
  domain_name       = "eamsa512.example.com"
  validation_method = "DNS"
  lifecycle {
    create_before_destroy = true
  }

  tags = {
    Name = "eamsa512-cert"
  }
}

# Launch Template
resource "aws_launch_template" "eamsa512" {
  name_prefix   = "eamsa512-"
  image_id      = data.aws_ami.amazon_linux_2.id
  instance_type = var.instance_type

  block_device_mappings {
    device_name = "/dev/xvda"

    ebs {
      volume_size           = 20
      volume_type           = "gp3"
      delete_on_termination = true
      encrypted             = true
    }
  }

  iam_instance_profile {
    name = aws_iam_instance_profile.eamsa512.name
  }

  security_groups = [aws_security_group.instance.id]

  user_data = base64encode(file("${path.module}/user_data.sh"))

  tag_specifications {
    resource_type = "instance"
    tags = {
      Name = "eamsa512-instance"
    }
  }

  lifecycle {
    create_before_destroy = true
  }
}

# Auto Scaling Group
resource "aws_autoscaling_group" "eamsa512" {
  name                = "eamsa512-asg"
  vpc_zone_identifier = aws_subnet.private[*].id
  target_group_arns   = [aws_lb_target_group.eamsa512.arn]
  health_check_type   = "ELB"
  health_check_grace_period = 300

  min_size         = 1
  max_size         = 6
  desired_capacity = var.desired_capacity

  launch_template {
    id      = aws_launch_template.eamsa512.id
    version = "$Latest"
  }

  tag {
    key                 = "Name"
    value               = "eamsa512-asg"
    propagate_at_launch = true
  }

  lifecycle {
    create_before_destroy = true
  }
}

# Auto Scaling Policies
resource "aws_autoscaling_policy" "scale_up" {
  name                   = "eamsa512-scale-up"
  autoscaling_group_name = aws_autoscaling_group.eamsa512.name
  adjustment_type        = "PercentChangeInCapacity"
  policy_type            = "TargetTrackingScaling"

  target_tracking_configuration {
    predefined_metric_specification {
      predefined_metric_type = "ASGAverageCPUUtilization"
    }
    target_value = 70.0
  }
}

# CloudWatch Alarms
resource "aws_cloudwatch_metric_alarm" "alb_unhealthy" {
  alarm_name          = "eamsa512-alb-unhealthy-hosts"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 2
  metric_name         = "UnHealthyHostCount"
  namespace           = "AWS/ApplicationELB"
  period              = 60
  statistic           = "Average"
  threshold           = 1
  alarm_description   = "Alert when ALB has unhealthy hosts"

  dimensions = {
    LoadBalancer = aws_lb.eamsa512.arn_suffix
    TargetGroup  = aws_lb_target_group.eamsa512.arn_suffix
  }
}

# RDS Database
resource "aws_db_subnet_group" "eamsa512" {
  name       = "eamsa512-db-subnet"
  subnet_ids = aws_subnet.private[*].id

  tags = {
    Name = "eamsa512-db-subnet"
  }
}

resource "aws_security_group" "rds" {
  name        = "eamsa512-rds"
  description = "RDS security group"
  vpc_id      = aws_vpc.eamsa512.id

  ingress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.instance.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "eamsa512-rds-sg"
  }
}

resource "aws_db_instance" "eamsa512" {
  identifier           = "eamsa512-db"
  engine               = "postgres"
  engine_version       = "15"
  instance_class       = "db.t3.micro"
  allocated_storage    = 20
  storage_encrypted    = true
  db_subnet_group_name = aws_db_subnet_group.eamsa512.name
  vpc_security_group_ids = [aws_security_group.rds.id]

  db_name  = "eamsa512"
  username = "eamsa512"
  password = random_password.db_password.result

  backup_retention_period = 30
  backup_window          = "03:00-04:00"
  maintenance_window     = "mon:04:00-mon:05:00"

  skip_final_snapshot = false
  final_snapshot_identifier = "eamsa512-final-snapshot"

  tags = {
    Name = "eamsa512-db"
  }
}

resource "random_password" "db_password" {
  length  = 32
  special = true
}

# Outputs
output "alb_dns_name" {
  value       = aws_lb.eamsa512.dns_name
  description = "DNS name of the load balancer"
}

output "db_endpoint" {
  value       = aws_db_instance.eamsa512.endpoint
  description = "RDS database endpoint"
}

output "rds_password_ssm_parameter" {
  value = aws_ssm_parameter.rds_password.name
}

# SSM Parameter for RDS Password
resource "aws_ssm_parameter" "rds_password" {
  name  = "/eamsa512/db/password"
  type  = "SecureString"
  value = random_password.db_password.result
}

# Data source for AMI
data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_ami" "amazon_linux_2" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["amzn2-ami-hvm-*-x86_64-gp2"]
  }
}

# IAM Role
resource "aws_iam_role" "eamsa512" {
  name = "eamsa512-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "ec2.amazonaws.com"
        }
      }
    ]
  })
}

resource "aws_iam_role_policy" "eamsa512" {
  name = "eamsa512-policy"
  role = aws_iam_role.eamsa512.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ssm:GetParameter",
          "ssm:GetParameters"
        ]
        Resource = "arn:aws:ssm:*:*:parameter/eamsa512/*"
      },
      {
        Effect = "Allow"
        Action = [
          "cloudwatch:PutMetricData",
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents"
        ]
        Resource = "*"
      }
    ]
  })
}

resource "aws_iam_instance_profile" "eamsa512" {
  name = "eamsa512-profile"
  role = aws_iam_role.eamsa512.name
}

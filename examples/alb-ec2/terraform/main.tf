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

# VPC
resource "aws_vpc" "main" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name = "alb-ec2-vpc"
  }
}

# Internet Gateway
resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id

  tags = {
    Name = "alb-ec2-igw"
  }
}

# Public Subnets
resource "aws_subnet" "public" {
  count                   = 2
  vpc_id                  = aws_vpc.main.id
  cidr_block              = "10.0.${count.index + 1}.0/24"
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  map_public_ip_on_launch = true

  tags = {
    Name = "alb-ec2-public-subnet-${count.index + 1}"
  }
}

# Route Table
resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }

  tags = {
    Name = "alb-ec2-public-rt"
  }
}

# Route Table Associations
resource "aws_route_table_association" "public" {
  count          = 2
  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

# Security Group for ALB
resource "aws_security_group" "alb" {
  name        = "alb-ec2-alb-sg"
  description = "Security group for ALB"
  vpc_id      = aws_vpc.main.id

  ingress {
    description = "HTTP"
    from_port   = 80
    to_port     = 80
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
    Name = "alb-ec2-alb-sg"
  }
}

# Security Group for EC2
resource "aws_security_group" "ec2" {
  name        = "alb-ec2-ec2-sg"
  description = "Security group for EC2 instances"
  vpc_id      = aws_vpc.main.id

  ingress {
    description     = "HTTP from ALB"
    from_port       = 80
    to_port         = 80
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
    Name = "alb-ec2-ec2-sg"
  }
}

# Application Load Balancer
resource "aws_lb" "main" {
  name               = "alb-ec2-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = aws_subnet.public[*].id

  enable_deletion_protection = false

  tags = {
    Name = "alb-ec2-alb"
  }
}

# Target Group
resource "aws_lb_target_group" "main" {
  name     = "alb-ec2-tg"
  port     = 80
  protocol = "HTTP"
  vpc_id   = aws_vpc.main.id

  health_check {
    enabled             = true
    healthy_threshold   = 2
    unhealthy_threshold = 2
    timeout             = 5
    interval            = 30
    path                = "/health"
    protocol            = "HTTP"
    matcher             = "200"
  }

  tags = {
    Name = "alb-ec2-tg"
  }
}

# ALB Listener
resource "aws_lb_listener" "main" {
  load_balancer_arn = aws_lb.main.arn
  port              = "80"
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.main.arn
  }
}

# EC2 Instances
data "aws_ami" "al2023" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["al2023-ami-*-x86_64"]
  }
}

resource "aws_instance" "app" {
  count                       = var.instance_count
  ami                         = data.aws_ami.al2023.id
  instance_type               = var.instance_type
  subnet_id                   = aws_subnet.public[count.index % 2].id
  vpc_security_group_ids      = [aws_security_group.ec2.id]
  associate_public_ip_address = true

  user_data = <<-EOF
              #!/bin/bash
              set -euxo pipefail
              
              # Install and start simple HTTP server
              cat >/home/ec2-user/index.html <<'HTML'
              <html>
                <head><title>ALB EC2 App</title></head>
                <body style="font-family: sans-serif;">
                  <h1>Hello from EC2!</h1>
                  <p>Instance: $(hostname)</p>
                  <p>Time: $(date)</p>
                </body>
              </html>
              HTML
              
              cat >/home/ec2-user/health.html <<'HTML'
              <html><body>OK</body></html>
              HTML
              
              # systemd service for main app
              cat >/etc/systemd/system/app.service <<'SERVICE'
              [Unit]
              Description=Simple HTTP app
              After=network.target
              
              [Service]
              Type=simple
              User=ec2-user
              WorkingDirectory=/home/ec2-user
              ExecStart=/usr/bin/python3 -m http.server 80 --bind 0.0.0.0
              Restart=always
              
              [Install]
              WantedBy=multi-user.target
              SERVICE
              
              systemctl daemon-reload
              systemctl enable --now app.service
              EOF

  tags = {
    Name = "alb-ec2-app-${count.index + 1}"
  }
}

# Attach instances to target group
resource "aws_lb_target_group_attachment" "main" {
  count            = var.instance_count
  target_group_arn = aws_lb_target_group.main.arn
  target_id        = aws_instance.app[count.index].id
  port             = 80
}


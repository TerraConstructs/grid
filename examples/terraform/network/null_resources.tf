terraform {
  required_providers {
    null = {
      source = "hashicorp/null"
      version = "3.2.4"
    }
  }
}

resource "null_resource" "example" {
  triggers = {
    timestamp = timestamp()
  }
}

resource "null_resource" "another" {
  triggers = {
    value = "initial"
  }
}

output "vpc_id" {
  value = null_resource.example.id
}

output "subnet_id" {
  value = null_resource.another.id
}

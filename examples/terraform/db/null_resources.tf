terraform {
  required_providers {
    null = {
      source = "hashicorp/null"
      version = "3.2.4"
    }
  }
}

resource "null_resource" "db" {
  triggers = {
    value = local.network_vpc_id
  }
}


output "endpoint" {
  value = null_resource.db.id
}

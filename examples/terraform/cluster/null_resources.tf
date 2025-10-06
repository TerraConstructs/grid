terraform {
  required_providers {
    null = {
      source = "hashicorp/null"
      version = "3.2.4"
    }
  }
}

resource "null_resource" "cluster" {
  triggers = {
    value = local.network_vpc_id
  }
}


output "cluster_id" {
  value = null_resource.cluster.id
}

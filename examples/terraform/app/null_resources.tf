terraform {
  required_providers {
    null = {
      source = "hashicorp/null"
      version = "3.2.4"
    }
  }
}

resource "null_resource" "app" {
  triggers = {
    value = "${local.db_endpoint}-${local.cluster_cluster_id}"
  }
}


output "url" {
  value = null_resource.app.id
}

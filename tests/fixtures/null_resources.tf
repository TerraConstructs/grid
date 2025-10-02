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

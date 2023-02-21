terraform {
  required_providers {
    anomalo = {
      source = "squareup.com/custom/anomalo"
      version = "1.0.2"
    }
  }
}

provider "anomalo" {
  host = "https://anomalo.example.com"
  token = "<token>"
}

terraform {
  required_providers {
    anomalo = {
      source = "square/anomalo"
    }
  }
}

provider "anomalo" {
  host = "https://anomalo.example.com"
  token = "<token>"
}

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

### With organizations

terraform {
  required_providers {
    anomalo = {
      source = "square/anomalo"
    }
  }
}

provider "anomalo" {
  alias="square"
  host = "https://anomalo.example.com"
  token = "<token>"
  organization = "square"
}

provider "anomalo" {
  alias="cash-app"
  host = "https://anomalo.example.com"
  token = "<aDifferentToken>"
  organization = "cashapp"
}

resource "anomalo_table" "VariationsTable" {
  # <some attributes>
  provider = anomalo.square
}

resource "anomalo_table" "BitcoinPurchasesTable" {
  # <some attributes>
  provider = anomalo.cashapp
}

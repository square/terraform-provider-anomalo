# Terraform-Provider-Anomalo

A Terraform provider that allows you to manage [Anomalo](https://www.anomalo.com/) resources in bulk.

Benefits:
- Version controlled, declarative configuration
- Bulk actions (ex with `for_each`)
- Line-by-line diffs
- Collaboration, via cloud based state and resource locks to keep you from overwriting a teammate's work
- Variables
  - ex. update the notification time for all checks with a single variable change
- Use the same (or similar) check across tables & environments
- Integrate with your existing workflow

Full provider documentation is here. (TODO link)
See [`example.tf`](https://github.com/square/terraform-provider-anomalo/blob/master/examples/example.tf) for a sample configuration.

## Table of Contents

- [Terraform-Provider-Anomalo](#terraform-provider-anomalo)
  - [Table of Contents](#table-of-contents)
  - [Installation](#installation)
  - [Getting Started](#getting-started)
    - [Quick Start](#quick-start)
    - [Example Configuration](#example-configuration)
  - [Importing Resources](#importing-resources)
    - [Importing a Single Anomalo Table or Check](#importing-a-single-anomalo-table-or-check)
    - [Importing All Checks for a Table](#importing-all-checks-for-a-table)


## Installation
First, [install Terraform](https://developer.hashicorp.com/terraform/downloads).

To use the provider, include it in your Terraform configuration and run `terraform init`.

```terraform
terraform {
  required_providers {
    anomalo = {
      source = "square/anomalo" # TODO verify
    }
  }
}
```

You'll need to provide an API token and instance host to authenticate with Anomalo. These can be provided explicitly or in environment variables `ANOMALO_INSTANCE_HOST` and `ANOMALO_API_SECRET_TOKEN`. Running `terraform plan` should output no errors if the plugin connects successfully.


## Getting Started
### Quick Start

This section shows a minimal example that connects to Anomalo and configures a new table with terraform.

Copy the following terraform code into a new or existing terraform directory, replacing values where necessary:

```terraform
# Tell Terraform to install the provider
terraform {
  required_providers {
    anomalo = {
      source = "square/anomalo" # TODO
    }
  }
}

# Provide credentials
provider "anomalo" {
  host = "https://anomalo.example.com" # Your Anomalo host
  token = "<token>" # Your Anomalo API token
}

# Configure the table
resource "anomalo_table" "MyTable" {
    # Replace with a table you want to configure. It must already be accesible by Anomalo but not be configured.
    table_name                    = "square.items.variations" # Must include the warehouse name (in this case, Square)
    check_cadence_type            = "daily" # Whether to run checks at a set time (daily) or on arrival
    check_cadence_run_at_duration = "PT6H" # 6 AM Pacific Time
    always_alert_on_errors        = true
}
```

1. Run `terraform init` (if you haven't already)
2. Run `terraform plan` to preview changes
3. Run `terraform apply` to configure the table in Anomalo

This table is now tracked by terraform. If you make changes in the Anomalo UI or API, Terraform will notify you what changed when you run `terraform plan`.

To see a complete configuration, jump to [this section](#example-configuration).

To import tables and checks that have already been created in Anomalo, jump to [this section](#importing-resources).

### Example Configuration

See `examples/example.tf` [here](https://github.com/square/terraform-provider-anomalo/blob/master/examples/example.tf).

## Importing Resources

### Importing a Single Anomalo Table or Check

To manage resources that are already configured in Anomalo you must import them. Both tables and checks implement `terraform import`.

See the plugin documentation for details on each resource. (TODO docs link)

### Importing All Checks for a Table

To import a table (or multiple tables) and all of its existing checks, use the Python script provided in the `bootstrap` folder. This script will import the state for each table and all of its checks, and write the configuration to a `.tf` file (one file per table).

1. Download the `bootstrap` folder contents into the root of your terraform directory.
2. [Optional] Create a virtual environment `python -m venv env && source env/bin/activate`
3. Run `pip install -r requirements.txt`
4. Create a file `tables.txt` with a line-separated list of tables to bootstrap.
   1. Table names should be fully qualified with the warehouse name - i.e. `square.items.variations`
5.Set `ANOMALO_INSTANCE_HOST` and `ANOMALO_API_SECRET_TOKEN` environment variables
   1. Optionally set them via command line arugments: `--anomalo-host` and `--anomalo-token`
1. Run `python3 download_tables.py --table-file tables.txt`

After executing the script, you should have one `.tf` file per table. Run `terraform plan` to make sure it worked. You may need to make some configuration updates manually.

--

Brought to you by Square <img src="https://avatars.githubusercontent.com/u/82592" alt="GitHub logo" width="20" style="float: left; margin-right: 5px;"/>

# Contributing

We welcome contributions. For small changes, open a pull request. If you'd like to discuss an improvement or fix before writing the code, please open a Github issue.

Before code can be accepted all contributors must complete our [Individual Contributor License Agreement (CLA)](https://spreadsheets.google.com/spreadsheet/viewform?formkey=dDViT2xzUHAwRkI3X3k5Z0lQM091OGc6MQ&ndplr=1)

## Reporting Issues

Please report issues in Github issues.

# Table of Contents

- [Contributing](#contributing)
  - [Reporting Issues](#reporting-issues)
- [Table of Contents](#table-of-contents)
  - [Technical Details](#technical-details)
    - [Local Development](#local-development)
    - [Building](#building)
    - [Generating Documentation](#generating-documentation)
  - [Design Decisions](#design-decisions)
    - [Managing Checks and Tables Separately](#managing-checks-and-tables-separately)
    - [Client Side Filtering](#client-side-filtering)
    - [Equating Empty and Null Strings](#equating-empty-and-null-strings)
  - [Not Implemented/Future Work](#not-implementedfuture-work)


## Technical Details
### Local Development
1. Clone the repo
2. Run `go get && go mod tidy`
3. Add the following to your `~/.terraformrc` file to instruct Terraform to look at local files for providers with this name:
```shell
dev_overrides {
   "square/anomalo" = "/Users/jake/go/bin" # TODO update this after deciding on a package name in terraform registry
}
```
1. Run `go install .` Terraform will now use your local code.

### Building
1. Run `git tag v1.x.x` based on the most recent release in github.
2. Run `git push origin v1.x.x` 
3. A github actions workflow should start
4. Anything for registry? # TODO

### Generating Documentation
Use [terraform-plugin-docs](https://github.com/hashicorp/terraform-plugin-docs) to keep documentation in sync with plugin code.

## Design Decisions

We generally followed recommendations [from terraform](https://developer.hashicorp.com/terraform/plugin/best-practices).

### Managing Checks and Tables Separately
Checks and tables are separate resources. This means if a net new check is added to a table outside of terraform, terraform won't know unless it's explicitly imported.

We decided on this approach because:

- It follows Terraform [design principles](https://developer.hashicorp.com/terraform/plugin/best-practices/hashicorp-provider-design-principles#resources-should-represent-a-single-api-object)
- Managing a subset of checks is a valid use case (ex. a central data team applying checks to all tables in the organization)
- It's easier - terraform only supports identifying elements in a set (checks in a table) by the object hash. Ideally we could identify a check by its `check_static_id`. The consequence is that diffs are less informative, and it's harder (if possible?) to maintain `check_static_id` across updates.

### Client Side Filtering
There are several places we fetch an entire list of objects, and filter for the element of interest. This usually done for at least one of two reasons:
1. To simplify configuration schema at the expense of performance (ex. supporting check updates)
2. Anomalo's API doesn't support anything else (ex. fetching a notification channel by name)

### Equating Empty and Null Strings
Anomalo doesn't differentiate between the empty string ("") and null. To support more accurate diffs, this plugin doesn't differentiate either.

We manage this at several places where the state of strings enter & exit the program:
1. Configuration (via `planmodifier`s)
2. State (stored as "")
3. Read method
4. Write methods (Create, Update)

## Not Implemented/Future Work

- Add a resource or module that tracks all checks for a table.
  - Currently, terraform won't know if a net new check is added to a table outside of terraform
  - Detailed explanation for this design decision is [here] TODO
- Create one resource per check type
  - Currently, all check types are implemented via the `anomalo_check` resource with a free-form Params map
  - This would give better type checking, error messages, and validation if we had one resource per check type
- Implement more detailed attribute validation.
  - We could make sure values (and combinations of values) satisfy certain rules to fail earlier
  - We currently rely on Anomalo for most validation
- Add integration Tests
- Integrate with Anomalo CLI
  - Short term - bootstrapping config based on the generated YAML files
  - Long term - CLI with state managed by terraform for more informative diffs

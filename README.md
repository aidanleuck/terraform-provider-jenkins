# Terraform Provider for Jenkins

A modern, feature-rich Terraform provider for managing Jenkins infrastructure built with the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework).

## Why This Provider?

This Jenkins provider stands out for several compelling reasons:

🏗️ **Modern Architecture**: Built on the latest Terraform Plugin Framework, ensuring excellent performance, type safety, and future compatibility.

🔐 **Comprehensive Security**: Supports multiple authentication methods including username/password and custom CA certificates for self-signed SSL setups.

🖥️ **Flexible Node Management**: Complete support for both SSH and JNLP Jenkins agents with extensive configuration options:
- SSH agents with custom Java paths, JVM options, and connection parameters
- JNLP agents with WebSocket support and workspace configuration
- Automatic secret management for secure agent connections

⚡ **Developer Experience**: Clean, well-documented codebase with:
- Comprehensive test coverage using Docker containers
- Auto-generated documentation
- Clear separation of concerns
- Professional error handling and logging

🔧 **Production Ready**: Includes proper configuration validation, retry logic, and robust error handling to ensure reliable infrastructure management.

This provider bridges the gap between Jenkins administration and Infrastructure as Code, making it easy to manage Jenkins nodes declaratively alongside your other infrastructure resources.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.19

## Building The Provider

1. Clone the repository
1. Enter the repository directory
1. Build the provider using the Go `install` command:

```shell
go install
```

## Adding Dependencies

This provider uses [Go modules](https://github.com/golang/go/wiki/Modules).
Please see the Go documentation for the most up to date information about using Go modules.

To add a new dependency `github.com/author/dependency` to your Terraform provider:

```shell
go get github.com/author/dependency
go mod tidy
```

Then commit the changes to `go.mod` and `go.sum`.

## Using the provider

This provider enables you to manage Jenkins infrastructure as code. Here's a quick example:

```hcl
terraform {
  required_providers {
    jenkins = {
      source = "aidanleuck/jenkins"
    }
  }
}

provider "jenkins" {
  url      = "https://jenkins.example.com"
  username = var.jenkins_username
  password = var.jenkins_password
}

# Create an SSH-based Jenkins node
resource "jenkins_node" "build_agent" {
  name      = "build-agent-01"
  remote_fs = "/home/jenkins"
  executors = 2
  labels    = ["linux", "docker", "build"]
  
  launcher_configuration {
    type = "ssh"
    ssh_options {
      host         = "10.0.1.100"
      credentials_id = "ssh-key-credential"
      java_path    = "/usr/bin/java"
    }
  }
}
```

For more detailed examples and configuration options, see the [documentation](docs/).

## Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements) above).

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

To generate or update documentation, run `go generate`.

In order to run the full suite of Acceptance tests, run `make testacc`.

*Note:* Acceptance tests create real resources, and often cost money to run.

```shell
make testacc
```

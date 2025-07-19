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

# Example: SSH-based Jenkins node
resource "jenkins_node" "ssh_agent" {
  name        = "ssh-build-agent"
  description = "SSH-based build agent for CI/CD"
  remote_fs   = "/home/jenkins"
  executors   = 2
  labels      = ["linux", "docker", "build"]
  
  launcher_configuration {
    type = "ssh"
    ssh_options {
      host                   = "10.0.1.100"
      port                   = 22
      credentials_id         = "ssh-key-credential"
      java_path             = "/usr/bin/java"
      launch_timeout_seconds = 60
      max_num_retries       = 3
    }
  }
}

# Example: JNLP-based Jenkins node
resource "jenkins_node" "jnlp_agent" {
  name        = "jnlp-build-agent"
  description = "JNLP-based build agent for flexible deployment"
  remote_fs   = "/opt/jenkins"
  executors   = 1
  labels      = ["windows", "dotnet"]
  
  launcher_configuration {
    type = "jnlp"
    jnlp_options {
      use_web_socket           = true
      workdir_disabled         = false
      fail_if_workdir_missing  = true
      remoting_dir            = "/opt/jenkins/remoting"
    }
  }
}

# Data source example: Read existing node information
data "jenkins_node" "existing_node" {
  name = "existing-agent"
}

# Variables
variable "jenkins_username" {
  description = "Jenkins username"
  type        = string
  sensitive   = true
}

variable "jenkins_password" {
  description = "Jenkins password"
  type        = string
  sensitive   = true
}
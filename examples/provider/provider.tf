terraform {
  required_providers {
    jenkins-provider = {
      source = "aidaleuc/jenkins-provider"
    }
  }
}

provider "jenkins-provider" {
  url = "http://127.0.0.1:5000"
  username = ""
  password = ""
}
data "jenkins-provider_node" "test" {
  name = jenkins-provider_node.test.name
}

resource "jenkins-provider_node" "test"{
	name = "test-node"
	remote_fs = "C:\\filesystem"
	launcher_configuration = {
		launch_type = "jnlp"
	}
}

output "test-out" {
  value = resource.jenkins-provider_node.test.name
}



terraform {
  required_providers {
    jenkins-provider = {
      source = "aidaleuc/jenkins-provider"
    }
  }
}

provider "jenkins-provider" {
  url = "http://127.0.0.1:5000"
  username = "aidaleuc"
  password = "11d3433d67057c2e26bc273d1045b1e771"
}
resource "jenkins-provider_node" "jenkins-node" {
  name = "Hello-54"
  remote_fs = "C:\\_jenkins"
  executors = 5
  labels = ["hello", "judith"]
  description = "Long har stinky dicks are yummmmy"
  launcher_configuration = {
     launch_type = "ssh"
  }
}



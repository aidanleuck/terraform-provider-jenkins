// The provider package implements a Jenkins provider for Terraform
//
// node_data_source_test.go implements a set of tests to validate the data source
// matches the terraform resource configuration.
package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Terraform configuration for basic node test acc
const testNodeDataSourceBasicConfig = `
provider "jenkins-provider" {
	url = "http://127.0.0.1:5000"
}

data "jenkins-provider_node" "test-data" {
  name = jenkins-provider_node.test.name
}

resource "jenkins-provider_node" "test"{
	name = "test-node"
	remote_fs = "C:\\filesystem"
	labels = ["hello", "world"]
	description = "hello"
	executors = 1
	launcher_configuration = {
		type = "jnlp"
	}
}
`

// Name of the node resource and data source.
const (
	testNodeResourceName = "jenkins-provider_node.test"
	testDataSourceName   = "data.jenkins-provider_node.test-data"
)

// Test data source basic, checks the data source contains all common
// fields between node types
func TestAccNodeDataSourceBasic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testNodeDataSourceBasicConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(testNodeResourceName, "name",
						testDataSourceName, "name"),
					resource.TestCheckResourceAttrPair(testNodeResourceName, "remote_fs",
						testDataSourceName, "remote_fs"),
					resource.TestCheckResourceAttrPair(testNodeResourceName, "launcher_configuration.type",
						testDataSourceName, "launcher_configuration.type"),
					resource.TestCheckResourceAttrPair(testNodeResourceName, "executors",
						testDataSourceName, "executors"),
					resource.TestCheckResourceAttrPair(testNodeResourceName, "description",
						testDataSourceName, "description"),
					resource.TestCheckResourceAttrPair(testNodeResourceName, "jnlp_secret",
						testDataSourceName, "jnlp_secret"),
				),
			},
		},
	})
}

// Terraform configuration for ssh test.
const testNodeDataSourceSSHConfig = `
provider "jenkins-provider" {
	url = "http://127.0.0.1:5000"
}

data "jenkins-provider_node" "test-data" {
  name = jenkins-provider_node.test.name
}

resource "jenkins-provider_node" "test"{
	name = "test-node"
	remote_fs = "C:\\filesystem"
	launcher_configuration = {
		type = "ssh"
		ssh_options = {
			host = "127.0.0.1"
			port = 22
			launch_timeout_seconds = 30
			max_num_retries = 5
			retry_wait_time = 6
			jvm_options = "-Done"
			java_path = "/var/lib"
			prefix_start_slave_cmd = "woop"
			suffix_start_slave_cmd = "yoop"
		}
	}
}
`

// TestAccNodeDataSourceSSH makes sure the ssh launcher options match the provided Terraform configuration.
func TestAccNodeDataSourceSSH(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testNodeDataSourceSSHConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(testNodeResourceName, "launcher_configuration.type",
						testDataSourceName, "launcher_configuration.type"),
					resource.TestCheckResourceAttrPair(testNodeResourceName, "launcher_configuration.ssh_options.host",
						testDataSourceName, "launcher_configuration.ssh_options.host"),
					resource.TestCheckResourceAttrPair(testNodeResourceName, "launcher_configuration.ssh_options.port",
						testDataSourceName, "launcher_configuration.ssh_options.port"),
					resource.TestCheckResourceAttrPair(testNodeResourceName, "launcher_configuration.ssh_options.launch_timeout_seconds",
						testDataSourceName, "launcher_configuration.ssh_options.launch_timeout_seconds"),
					resource.TestCheckResourceAttrPair(testNodeResourceName, "launcher_configuration.ssh_options.max_num_retries",
						testDataSourceName, "launcher_configuration.ssh_options.max_num_retries"),
					resource.TestCheckResourceAttrPair(testNodeResourceName, "launcher_configuration.ssh_options.retry_wait_time",
						testDataSourceName, "launcher_configuration.ssh_options.retry_wait_time"),
					resource.TestCheckResourceAttrPair(testNodeResourceName, "launcher_configuration.ssh_options.jvm_options",
						testDataSourceName, "launcher_configuration.ssh_options.jvm_options"),
					resource.TestCheckResourceAttrPair(testNodeResourceName, "launcher_configuration.ssh_options.java_path",
						testDataSourceName, "launcher_configuration.ssh_options.java_path"),
					resource.TestCheckResourceAttrPair(testNodeResourceName, "launcher_configuration.ssh_options.prefix_start_slave_cmd",
						testDataSourceName, "launcher_configuration.ssh_options.prefix_start_slave_cmd"),
					resource.TestCheckResourceAttrPair(testNodeResourceName, "launcher_configuration.ssh_options.suffix_start_slave_cmd",
						testDataSourceName, "launcher_configuration.ssh_options.suffix_start_slave_cmd"),
				),
			},
		},
	})
}

// Terraform configuraiton for testing a JNLP data source.
const testNodeDataSourceJNLPConfig = `
provider "jenkins-provider" {
	url = "http://127.0.0.1:5000"
}

data "jenkins-provider_node" "test-data" {
  name = jenkins-provider_node.test.name
}

resource "jenkins-provider_node" "test"{
	name = "test-node"
	remote_fs = "C:\\filesystem"
	launcher_configuration = {
		type = "jnlp"
		jnlp_options = {
			use_web_socket = true
			workdir_disabled = true
			fail_if_workdir_missing = true
			remoting_dir = true
		}
	}
}
`

// TestAccNodeDataSourceJNLP tests jnlp launcher options and make sure they match the
// created terraform resource.
func TestAccNodeDataSourceJNLP(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testNodeDataSourceJNLPConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(testNodeResourceName, "launcher_configuration.type",
						testDataSourceName, "launcher_configuration.type"),
					resource.TestCheckResourceAttrPair(testNodeResourceName, "launcher_configuration.jnlp_options.use_web_socket",
						testDataSourceName, "launcher_configuration.jnlp_options.use_web_socket"),
					resource.TestCheckResourceAttrPair(testNodeResourceName, "launcher_configuration.jnlp_options.workdir_disabled",
						testDataSourceName, "launcher_configuration.jnlp_options.workdir_disabled"),
					resource.TestCheckResourceAttrPair(testNodeResourceName, "launcher_configuration.jnlp_options.remoting_dir",
						testDataSourceName, "launcher_configuration.jnlp_options.remoting_dir"),
					resource.TestCheckResourceAttrPair(testNodeResourceName, "launcher_configuration.jnlp_options.fail_if_workdir_missing",
						testDataSourceName, "launcher_configuration.jnlp_options.fail_if_workdir_missing"),
				),
			},
		},
	})
}

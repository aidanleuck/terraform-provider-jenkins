provider "jenkins"{
    url = "http://localhost:8080"
    username = ""
    password = ""
}


resource "jenkins_node" "test" {
	name = "test_node_all_properties"
	executors = 2
	description = "Node with all properties combined"
	remote_fs = "/var/jenkins"
	labels = ["all-properties-test"]
	environment_variables = {
		"TEST_VAR" = "test_value"
	}

    // Note a git tool named default must be configured globally
	tool_locations = {
		"hudson.plugins.git.GitTool$DescriptorImpl:Default" = "/usr/bin/git"
	}

    // disk thresholds
	free_disk_space_threshold = "2GiB"
	free_temp_space_threshold = "1GiB"
	free_disk_space_warning_threshold = "1500MiB"
	free_temp_space_warning_threshold = "800MiB"
    disable_deferred_wipeout = false
	launcher_configuration = {
		type = "jnlp"	
	}
}
// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/aidanleuck/gojenkins"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const providerConfig = `
provider "jenkins" {
	url = "{{ .Server }}"
	username = "{{ .Username }}"
	password = "{{ .Password }}"
}
`

func TestBasicNode(t *testing.T) {
	resourceConfig := `
resource "jenkins_node" "test" {
	name = "{{ .Name }}"
	executors = {{ .Executors }}
	description = "{{ .Description }}"
	remote_fs = "{{ .RemoteFS }}"
	labels = {{ .TemplateLabel }}
	launcher_configuration = {
		type = "jnlp"	
    }
}
	`

	mergedConfig := providerConfig + "\n" + resourceConfig
	pd := getProviderData(testContainer)

	type testData struct {
		providerData
		TestName      string
		Name          string
		Executors     int
		Description   string
		RemoteFS      string
		Labels        []string
		TemplateLabel string
	}

	tcs := []testData{
		{
			providerData: *pd,
			TestName:     "basic_node",
			Name:         "basic_node",
			Executors:    1,
			Description:  "basic_node",
			RemoteFS:     "/tmp",
			Labels:       []string{"basic_node"},
		},
		{
			providerData: *pd,
			TestName:     "basic_node_2",
			Name:         "basic_node_2",
			Executors:    5,
			Description:  "basic_node_2",
			RemoteFS:     "/tmp",
			Labels:       []string{"basic_node_2"},
		},
		{
			providerData: *pd,
			TestName:     "node_without_labels",
			Name:         "basic_node_3",
			Executors:    1,
			Description:  "",
			RemoteFS:     "/tmp",
			Labels:       []string{},
		},
		{
			providerData: *pd,
			TestName:     "node_with_multiple_labels",
			Name:         "basic_node_4",
			Executors:    1,
			Description:  "",
			RemoteFS:     "/tmp",
			Labels:       []string{"label1", "label2", "label3"},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.TestName, func(t *testing.T) {
			tc.TemplateLabel = generateTemplateLabelStr(tc.Labels)
			labelTestFuncs := getLabelTestArrFunc(tc.Labels)
			fieldTestFuncs := []resource.TestCheckFunc{
				resource.TestCheckResourceAttr(testNodeResourceName, "name", tc.Name),
				resource.TestCheckResourceAttr(testNodeResourceName, "executors", strconv.Itoa(tc.Executors)),
				resource.TestCheckResourceAttr(testNodeResourceName, "description", tc.Description),
				resource.TestCheckResourceAttr(testNodeResourceName, "remote_fs", tc.RemoteFS),
				compareBasicNodeDetails(),
			}

			testFuncs := make([]resource.TestCheckFunc, 0, len(labelTestFuncs)+len(fieldTestFuncs))
			testFuncs = append(testFuncs, fieldTestFuncs...)
			testFuncs = append(testFuncs, labelTestFuncs...)
			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config: templateConfig(t, mergedConfig, tc),
						Check: resource.ComposeTestCheckFunc(
							testFuncs...,
						),
					},
					// Destroy the node
					{
						Config:  templateConfig(t, mergedConfig, tc),
						Destroy: true,
					},
					// Make sure the node is recreated
					{
						Config: templateConfig(t, mergedConfig, tc),
						Check: resource.ComposeTestCheckFunc(
							testFuncs...,
						),
					},
				},
			})
		})
	}
}

func TestJNLPNodeBasic(t *testing.T) {
	name := "test_node"
	executors := 1
	description := "test_node"
	remoteFS := "/tmp"

	resourceConfig := fmt.Sprintf(`
resource "jenkins_node" "test" {
	name = "%s"
	executors = %d
	description = "%s"
	remote_fs = "%s"
	labels = ["test_node"]
	launcher_configuration = {
		type = "jnlp"
    }
}
`, name, executors, description, remoteFS)

	mergedConfig := providerConfig + "\n" + resourceConfig
	pd := getProviderData(testContainer)

	type testData struct {
		providerData
	}

	td := testData{
		providerData: *pd,
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: templateConfig(t, mergedConfig, td),
				Check: resource.ComposeTestCheckFunc(
					compareBasicNodeDetails(),
					compareJNLPNodeBasic(),
					resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.type", "jnlp"),
					resource.TestCheckResourceAttrSet(testNodeResourceName, "jnlp_secret"),
				),
			},
			// Destroy the node
			{
				Config:  templateConfig(t, mergedConfig, td),
				Destroy: true,
			},
			// Make sure the node is recreated
			{
				Config: templateConfig(t, mergedConfig, td),
				Check: resource.ComposeTestCheckFunc(
					compareBasicNodeDetails(),
					compareJNLPNodeBasic(),
					resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.type", "jnlp"),
					resource.TestCheckResourceAttrSet(testNodeResourceName, "jnlp_secret"),
				),
			},
		},
	})
}

func TestJNLPNodeAdvanced(t *testing.T) {
	name := "test_node"
	executors := 1
	description := "test_node"
	remoteFS := "/tmp"

	resourceConfig := fmt.Sprintf(`
resource "jenkins_node" "test" {
	name = "%s"
	executors = %d
	description = "%s"
	remote_fs = "%s"
	labels = ["test_node"]
	launcher_configuration = {
		type = "jnlp"
		jnlp_options = {
			fail_if_workdir_missing = {{ .FailIfWorkDirMissing }}
			use_web_socket = {{ .UseWebSocket }}
			workdir_disabled = {{ .WorkdirDisabled }}
			remoting_dir = "{{ .RemotingDir }}"
		}
    }
}
`, name, executors, description, remoteFS)

	mergedConfig := providerConfig + "\n" + resourceConfig
	pd := getProviderData(testContainer)

	type testData struct {
		providerData
		TestName             string
		FailIfWorkDirMissing bool
		UseWebSocket         bool
		WorkdirDisabled      bool
		RemotingDir          string
	}

	tcs := []testData{
		{
			providerData:         *pd,
			FailIfWorkDirMissing: true,
			UseWebSocket:         true,
			WorkdirDisabled:      true,
			RemotingDir:          "/tmp",
			TestName:             "jnlp_node",
		},
		{
			providerData:         *pd,
			FailIfWorkDirMissing: false,
			UseWebSocket:         false,
			WorkdirDisabled:      false,
			RemotingDir:          "/tmp",
			TestName:             "jnlp_node_2",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.TestName, func(t *testing.T) {
			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config: templateConfig(t, mergedConfig, tc),
						Check: resource.ComposeTestCheckFunc(
							compareBasicNodeDetails(),
							compareJNLPLauncherSettingsAdvanced(),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.type", "jnlp"),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.jnlp_options.fail_if_workdir_missing", strconv.FormatBool(tc.FailIfWorkDirMissing)),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.jnlp_options.use_web_socket", strconv.FormatBool(tc.UseWebSocket)),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.jnlp_options.workdir_disabled", strconv.FormatBool(tc.WorkdirDisabled)),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.jnlp_options.remoting_dir", tc.RemotingDir),
							resource.TestCheckResourceAttrSet(testNodeResourceName, "jnlp_secret"),
						),
					},
					// Destroy the node
					{
						Config:  templateConfig(t, mergedConfig, tc),
						Destroy: true,
					},
					// Make sure the node is recreated
					{
						Config: templateConfig(t, mergedConfig, tc),
						Check: resource.ComposeTestCheckFunc(
							compareBasicNodeDetails(),
							compareJNLPLauncherSettingsAdvanced(),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.type", "jnlp"),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.jnlp_options.fail_if_workdir_missing", strconv.FormatBool(tc.FailIfWorkDirMissing)),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.jnlp_options.use_web_socket", strconv.FormatBool(tc.UseWebSocket)),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.jnlp_options.workdir_disabled", strconv.FormatBool(tc.WorkdirDisabled)),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.jnlp_options.remoting_dir", tc.RemotingDir),
							resource.TestCheckResourceAttrSet(testNodeResourceName, "jnlp_secret"),
						),
					},
				},
			})
		})
	}
}

func TestNodeSSHBasic(t *testing.T) {
	name := "test_node"
	executors := 1
	description := "test_node"
	remoteFS := "/tmp"

	resourceConfig := fmt.Sprintf(`
resource "jenkins_node" "test" {
	name = "%s"
	executors = %d
	description = "%s"
	remote_fs = "%s"
	labels = ["test_node"]
	launcher_configuration = {
		type = "ssh"
		ssh_options = {
			host = "localhost"
		}
    }
}
`, name, executors, description, remoteFS)

	mergedConfig := providerConfig + "\n" + resourceConfig
	pd := getProviderData(testContainer)

	type testData struct {
		providerData
	}

	td := testData{
		providerData: *pd,
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: templateConfig(t, mergedConfig, td),
				Check: resource.ComposeTestCheckFunc(
					compareBasicNodeDetails(),
					compareSSHLauncherBasic(),
					resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.type", "ssh"),
					resource.TestCheckNoResourceAttr(testNodeResourceName, "jnlp_secret"),
				),
			},
			// Destroy the node
			{
				Config:  templateConfig(t, mergedConfig, td),
				Destroy: true,
			},
			// Make sure the node is recreated
			{
				Config: templateConfig(t, mergedConfig, td),
				Check: resource.ComposeTestCheckFunc(
					compareBasicNodeDetails(),
					compareSSHLauncherBasic(),
					resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.type", "ssh"),
					resource.TestCheckNoResourceAttr(testNodeResourceName, "jnlp_secret"),
				),
			},
		},
	})
}

func TestSSHNodeAdvanced(t *testing.T) {
	name := "test_node"
	executors := 1
	description := "test_node"
	remoteFS := "/tmp"

	resourceConfig := fmt.Sprintf(`
resource "jenkins_node" "test" {
	name = "%s"
	executors = %d
	description = "%s"
	remote_fs = "%s"
	labels = ["test_node"]
	launcher_configuration = {
		type = "ssh"
		ssh_options = {
			host = "{{ .Host }}"
			port = {{ .Port }}
			credentials_id = "{{ .CredentialsId }}"
			java_path = "{{ .JavaPath }}"
			jvm_options = "{{ .JvmOptions }}"
			launch_timeout_seconds = {{ .LaunchTimeoutSeconds }}
			max_num_retries = {{ .MaxNumRetries }}
			retry_wait_time = {{ .RetryWaitTime }}
			suffix_start_slave_cmd = "{{ .SuffixStartSlaveCmd }}"
			prefix_start_slave_cmd = "{{ .PrefixStartSlaveCmd }}"
		}
    }
}
`, name, executors, description, remoteFS)

	mergedConfig := providerConfig + "\n" + resourceConfig
	pd := getProviderData(testContainer)

	type testData struct {
		providerData
		TestName             string
		Host                 string
		Port                 int
		CredentialsId        string
		JavaPath             string
		JvmOptions           string
		LaunchTimeoutSeconds int
		MaxNumRetries        int
		RetryWaitTime        int
		SuffixStartSlaveCmd  string
		PrefixStartSlaveCmd  string
	}

	tcs := []testData{
		{
			providerData:         *pd,
			Host:                 "mock_host",
			Port:                 9000,
			CredentialsId:        "mock_credentials_id",
			JavaPath:             "mock_java_path",
			JvmOptions:           "mock_jvm_options",
			LaunchTimeoutSeconds: 6000,
			MaxNumRetries:        5,
			RetryWaitTime:        10,
			SuffixStartSlaveCmd:  "mock_suffix_start_slave_cmd",
			PrefixStartSlaveCmd:  "mock_prefix_start_slave_cmd",
			TestName:             "ssh_node",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.TestName, func(t *testing.T) {
			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config: templateConfig(t, mergedConfig, tc),
						Check: resource.ComposeTestCheckFunc(
							compareBasicNodeDetails(),
							compareSSHLauncherAdvanced(),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.type", "ssh"),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.ssh_options.host", tc.Host),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.ssh_options.port", strconv.Itoa(tc.Port)),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.ssh_options.credentials_id", tc.CredentialsId),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.ssh_options.java_path", tc.JavaPath),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.ssh_options.jvm_options", tc.JvmOptions),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.ssh_options.launch_timeout_seconds", strconv.Itoa(tc.LaunchTimeoutSeconds)),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.ssh_options.max_num_retries", strconv.Itoa(tc.MaxNumRetries)),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.ssh_options.retry_wait_time", strconv.Itoa(tc.RetryWaitTime)),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.ssh_options.suffix_start_slave_cmd", tc.SuffixStartSlaveCmd),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.ssh_options.prefix_start_slave_cmd", tc.PrefixStartSlaveCmd),
							resource.TestCheckNoResourceAttr(testNodeResourceName, "jnlp_secret"),
						),
					},
					// Destroy the node
					{
						Config:  templateConfig(t, mergedConfig, tc),
						Destroy: true,
					},
					// Make sure the node is recreated
					{
						Config: templateConfig(t, mergedConfig, tc),
						Check: resource.ComposeTestCheckFunc(
							compareBasicNodeDetails(),
							compareSSHLauncherAdvanced(),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.type", "ssh"),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.ssh_options.host", tc.Host),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.ssh_options.port", strconv.Itoa(tc.Port)),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.ssh_options.credentials_id", tc.CredentialsId),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.ssh_options.java_path", tc.JavaPath),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.ssh_options.jvm_options", tc.JvmOptions),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.ssh_options.launch_timeout_seconds", strconv.Itoa(tc.LaunchTimeoutSeconds)),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.ssh_options.max_num_retries", strconv.Itoa(tc.MaxNumRetries)),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.ssh_options.retry_wait_time", strconv.Itoa(tc.RetryWaitTime)),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.ssh_options.suffix_start_slave_cmd", tc.SuffixStartSlaveCmd),
							resource.TestCheckResourceAttr(testNodeResourceName, "launcher_configuration.ssh_options.prefix_start_slave_cmd", tc.PrefixStartSlaveCmd),
							resource.TestCheckNoResourceAttr(testNodeResourceName, "jnlp_secret"),
						),
					},
				},
			})
		})
	}
}

func TestTypeRequired(t *testing.T) {
	resourceConfig := `
resource "jenkins_node" "test" {
	name = "test_node"
	executors = 1
	description = "test_node"
	remote_fs = "/tmp"
	labels = ["test_node"]
	launcher_configuration = {}
}`

	mergedConfig := providerConfig + "\n" + resourceConfig
	pd := getProviderData(testContainer)
	type testData struct {
		providerData
	}

	testConfig := templateConfig(t, mergedConfig, pd)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testConfig,
				ExpectError: regexp.MustCompile(".+Incorrect attribute.+"),
			},
		},
	})
}

func TestSSHHostRequired(t *testing.T) {
	resourceConfig := `
resource "jenkins_node" "test" {
	name = "test_node"
	executors = 1
	description = "test_node"
	remote_fs = "/tmp"
	labels = ["test_node"]
	launcher_configuration = {
		type = "ssh"
		ssh_options = {}
	}
}`

	mergedConfig := providerConfig + "\n" + resourceConfig
	pd := getProviderData(testContainer)
	type testData struct {
		providerData
	}

	testConfig := templateConfig(t, mergedConfig, pd)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testConfig,
				ExpectError: regexp.MustCompile(".+Incorrect attribute.+"),
			},
		},
	})
}

func generateTemplateLabelStr(labels []string) string {
	if len(labels) == 0 {
		return "[]"
	}

	labelsStr := "["
	for _, label := range labels {
		labelsStr += "\"" + label + "\","
	}
	labelsStr = labelsStr + "]"
	return labelsStr
}

func getLabelTestArrFunc(labels []string) []resource.TestCheckFunc {
	labelTestArr := make([]resource.TestCheckFunc, len(labels))
	for i, label := range labels {
		labelTestArr[i] = resource.TestCheckResourceAttr(testNodeResourceName, "labels."+strconv.Itoa(i), label)
	}
	return labelTestArr
}

func compareBasicNodeDetails() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		nodeName := s.RootModule().Resources[testNodeResourceName].Primary.Attributes["name"]
		node, err := testContainer.Jenkins.GetNode(context.Background(), nodeName)
		if err != nil {
			return err
		}

		config, err := node.GetSlaveConfig(context.Background())
		if err != nil {
			return err
		}

		if node.GetName() != nodeName {
			return fmt.Errorf("node name mismatch: expected %s, got %s", nodeName, node.GetName())
		}

		executors := s.RootModule().Resources[testNodeResourceName].Primary.Attributes["executors"]
		executorsInt, err := strconv.Atoi(executors)
		if err != nil {
			return fmt.Errorf("failed to convert executors to int: %s", err)
		}

		if config.NumExecutors != executorsInt {
			return fmt.Errorf("node executors mismatch: expected %s, got %d", executors, config.NumExecutors)
		}
		description := s.RootModule().Resources[testNodeResourceName].Primary.Attributes["description"]
		if config.Description != description {
			return fmt.Errorf("node description mismatch: expected %s, got %s", description, config.Description)
		}
		remoteFS := s.RootModule().Resources[testNodeResourceName].Primary.Attributes["remote_fs"]
		if config.RemoteFS != remoteFS {
			return fmt.Errorf("node remote_fs mismatch: expected %s, got %s", remoteFS, config.RemoteFS)
		}

		labelCount := s.RootModule().Resources[testNodeResourceName].Primary.Attributes["labels.#"]
		labelCountInt, err := strconv.Atoi(labelCount)
		if err != nil {
			return fmt.Errorf("failed to convert label count to int: %s", err)
		}

		nodeLabels := config.Label
		labelArr := strings.Split(nodeLabels, " ")
		if nodeLabels == "" {
			labelArr = []string{}
		}

		if labelCountInt != len(labelArr) {
			return fmt.Errorf("node label count mismatch: expected %d, got %d", labelCountInt, len(labelArr))
		}

		for i := 0; i < labelCountInt; i++ {
			label := s.RootModule().Resources[testNodeResourceName].Primary.Attributes[fmt.Sprintf("labels.%d", i)]
			if label != labelArr[i] {
				return fmt.Errorf("node label mismatch: expected %s, got %s", label, labelArr[i])
			}
		}

		return nil
	}
}

func compareJNLPLauncherSettingsAdvanced() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		nodeName := s.RootModule().Resources[testNodeResourceName].Primary.Attributes["name"]
		node, err := testContainer.Jenkins.GetNode(context.Background(), nodeName)
		if err != nil {
			return err
		}

		config, err := node.GetSlaveConfig(context.Background())
		if err != nil {
			return err
		}

		launcher, ok := config.Launcher.Launcher.(*gojenkins.JNLPLauncher)
		if !ok {
			return fmt.Errorf("node launcher is not a JNLPLauncher")
		}

		failIfWorkDirIsMissing := s.RootModule().Resources[testNodeResourceName].Primary.Attributes["launcher_configuration.jnlp_options.fail_if_workdir_missing"]
		failIfWorkDirIsMissingBool, err := strconv.ParseBool(failIfWorkDirIsMissing)
		if err != nil {
			return fmt.Errorf("failed to convert fail_if_workdir_missing to bool: %s", err)
		}

		useWebSocket := s.RootModule().Resources[testNodeResourceName].Primary.Attributes["launcher_configuration.jnlp_options.use_web_socket"]
		useWebSocketBool, err := strconv.ParseBool(useWebSocket)
		if err != nil {
			return fmt.Errorf("failed to convert use_web_socket to bool: %s", err)
		}

		workdirDisabled := s.RootModule().Resources[testNodeResourceName].Primary.Attributes["launcher_configuration.jnlp_options.workdir_disabled"]
		workdirDisabledBool, err := strconv.ParseBool(workdirDisabled)
		if err != nil {
			return fmt.Errorf("failed to convert workdir_disabled to bool: %s", err)
		}

		remotingDir := s.RootModule().Resources[testNodeResourceName].Primary.Attributes["launcher_configuration.jnlp_options.remoting_dir"]

		expectedLauncher := &gojenkins.JNLPLauncher{
			WorkDirSettings: &gojenkins.WorkDirSettings{
				FailIfWorkDirIsMissing: failIfWorkDirIsMissingBool,
				Disabled:               workdirDisabledBool,
				InternalDir:            remotingDir,
			},
			WebSocket: useWebSocketBool,
		}

		if err := compareJNLPSettings(launcher, expectedLauncher); err != nil {
			return err
		}

		return nil
	}
}

func compareJNLPSettings(actual *gojenkins.JNLPLauncher, expected *gojenkins.JNLPLauncher) error {
	if actual.WorkDirSettings.FailIfWorkDirIsMissing != expected.WorkDirSettings.FailIfWorkDirIsMissing {
		return fmt.Errorf("node fail_if_workdir_missing mismatch: expected %t, got %t", expected.WorkDirSettings.FailIfWorkDirIsMissing, actual.WorkDirSettings.FailIfWorkDirIsMissing)
	}
	if actual.WebSocket != expected.WebSocket {
		return fmt.Errorf("node use_web_socket mismatch: expected %t, got %t", expected.WebSocket, actual.WebSocket)
	}
	if actual.WorkDirSettings.Disabled != expected.WorkDirSettings.Disabled {
		return fmt.Errorf("node workdir_disabled mismatch: expected %t, got %t", expected.WorkDirSettings.Disabled, actual.WorkDirSettings.Disabled)
	}
	if actual.WorkDirSettings.InternalDir != expected.WorkDirSettings.InternalDir {
		return fmt.Errorf("node remoting_dir mismatch: expected %s, got %s", expected.WorkDirSettings.InternalDir, actual.WorkDirSettings.InternalDir)
	}
	return nil
}

func compareJNLPNodeBasic() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		nodeName := s.RootModule().Resources[testNodeResourceName].Primary.Attributes["name"]
		node, err := testContainer.Jenkins.GetNode(context.Background(), nodeName)
		if err != nil {
			return err
		}

		config, err := node.GetSlaveConfig(context.Background())
		if err != nil {
			return err
		}

		if config.Launcher.Launcher == nil {
			return fmt.Errorf("node launcher is nil")
		}

		if _, ok := config.Launcher.Launcher.(*gojenkins.JNLPLauncher); !ok {
			return fmt.Errorf("node launcher is not a JNLPLauncher")
		}

		referenceLauncher := gojenkins.DefaultJNLPLauncher()

		if err := compareJNLPSettings(config.Launcher.Launcher.(*gojenkins.JNLPLauncher), referenceLauncher); err != nil {
			return err
		}

		return nil
	}
}

func compareSSHLauncherBasic() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		nodeName := s.RootModule().Resources[testNodeResourceName].Primary.Attributes["name"]
		node, err := testContainer.Jenkins.GetNode(context.Background(), nodeName)
		if err != nil {
			return err
		}

		config, err := node.GetSlaveConfig(context.Background())
		if err != nil {
			return err
		}

		if config.Launcher.Launcher == nil {
			return fmt.Errorf("node launcher is nil")
		}

		if _, ok := config.Launcher.Launcher.(*gojenkins.SSHLauncher); !ok {
			return fmt.Errorf("node launcher is not a SSHLauncher")
		}

		referenceLauncher := gojenkins.DefaultSSHLauncher()
		referenceLauncher.Host = s.RootModule().Resources[testNodeResourceName].Primary.Attributes["launcher_configuration.ssh_options.host"]

		if err := compareSSHSettings(config.Launcher.Launcher.(*gojenkins.SSHLauncher), referenceLauncher); err != nil {
			return err
		}

		return nil
	}
}

func compareSSHLauncherAdvanced() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		nodeName := s.RootModule().Resources[testNodeResourceName].Primary.Attributes["name"]
		node, err := testContainer.Jenkins.GetNode(context.Background(), nodeName)
		if err != nil {
			return err
		}

		config, err := node.GetSlaveConfig(context.Background())
		if err != nil {
			return err
		}

		if config.Launcher.Launcher == nil {
			return fmt.Errorf("node launcher is nil")
		}

		if _, ok := config.Launcher.Launcher.(*gojenkins.SSHLauncher); !ok {
			return fmt.Errorf("node launcher is not a SSHLauncher")
		}

		host := s.RootModule().Resources[testNodeResourceName].Primary.Attributes["launcher_configuration.ssh_options.host"]
		port := s.RootModule().Resources[testNodeResourceName].Primary.Attributes["launcher_configuration.ssh_options.port"]
		portInt, err := strconv.Atoi(port)
		if err != nil {
			return fmt.Errorf("failed to convert port to int: %s", err)
		}
		credentialsId := s.RootModule().Resources[testNodeResourceName].Primary.Attributes["launcher_configuration.ssh_options.credentials_id"]
		javaPath := s.RootModule().Resources[testNodeResourceName].Primary.Attributes["launcher_configuration.ssh_options.java_path"]
		jvmOptions := s.RootModule().Resources[testNodeResourceName].Primary.Attributes["launcher_configuration.ssh_options.jvm_options"]
		launchTimeoutSeconds := s.RootModule().Resources[testNodeResourceName].Primary.Attributes["launcher_configuration.ssh_options.launch_timeout_seconds"]
		launchTimeoutSecondsInt, err := strconv.Atoi(launchTimeoutSeconds)
		if err != nil {
			return fmt.Errorf("failed to convert launch_timeout_seconds to int: %s", err)
		}
		maxNumRetries := s.RootModule().Resources[testNodeResourceName].Primary.Attributes["launcher_configuration.ssh_options.max_num_retries"]
		maxNumRetriesInt, err := strconv.Atoi(maxNumRetries)
		if err != nil {
			return fmt.Errorf("failed to convert max_num_retries to int: %s", err)
		}
		retryWaitTime := s.RootModule().Resources[testNodeResourceName].Primary.Attributes["launcher_configuration.ssh_options.retry_wait_time"]
		retryWaitTimeInt, err := strconv.Atoi(retryWaitTime)
		if err != nil {
			return fmt.Errorf("failed to convert retry_wait_time to int: %s", err)
		}
		suffixStartSlaveCmd := s.RootModule().Resources[testNodeResourceName].Primary.Attributes["launcher_configuration.ssh_options.suffix_start_slave_cmd"]
		prefixStartSlaveCmd := s.RootModule().Resources[testNodeResourceName].Primary.Attributes["launcher_configuration.ssh_options.prefix_start_slave_cmd"]

		referenceLauncher := &gojenkins.SSHLauncher{
			Host:                 host,
			Port:                 portInt,
			CredentialsId:        credentialsId,
			JavaPath:             javaPath,
			JvmOptions:           jvmOptions,
			LaunchTimeoutSeconds: launchTimeoutSecondsInt,
			MaxNumRetries:        maxNumRetriesInt,
			RetryWaitTime:        retryWaitTimeInt,
			SuffixStartSlaveCmd:  suffixStartSlaveCmd,
			PrefixStartSlaveCmd:  prefixStartSlaveCmd,
		}

		if err := compareSSHSettings(config.Launcher.Launcher.(*gojenkins.SSHLauncher), referenceLauncher); err != nil {
			return err
		}

		return nil
	}
}

func compareSSHSettings(actual *gojenkins.SSHLauncher, expected *gojenkins.SSHLauncher) error {
	if actual.Host != expected.Host {
		return fmt.Errorf("node host mismatch: expected %s, got %s", expected.Host, actual.Host)
	}
	if actual.Port != expected.Port {
		return fmt.Errorf("node port mismatch: expected %d, got %d", expected.Port, actual.Port)
	}
	if actual.CredentialsId != expected.CredentialsId {
		return fmt.Errorf("node username mismatch: expected %s, got %s", expected.CredentialsId, actual.CredentialsId)
	}

	if actual.JavaPath != expected.JavaPath {
		return fmt.Errorf("node java_path mismatch: expected %s, got %s", expected.JavaPath, actual.JavaPath)
	}

	if actual.JvmOptions != expected.JvmOptions {
		return fmt.Errorf("node jvm_options mismatch: expected %s, got %s", expected.JvmOptions, actual.JvmOptions)
	}

	if actual.LaunchTimeoutSeconds != expected.LaunchTimeoutSeconds {
		return fmt.Errorf("node launch_timeout_seconds mismatch: expected %d, got %d", expected.LaunchTimeoutSeconds, actual.LaunchTimeoutSeconds)
	}

	if actual.MaxNumRetries != expected.MaxNumRetries {
		return fmt.Errorf("node max_num_retries mismatch: expected %d, got %d", expected.MaxNumRetries, actual.MaxNumRetries)
	}

	if actual.RetryWaitTime != expected.RetryWaitTime {
		return fmt.Errorf("node retry_wait_time mismatch: expected %d, got %d", expected.RetryWaitTime, actual.RetryWaitTime)
	}

	if actual.SuffixStartSlaveCmd != expected.SuffixStartSlaveCmd {
		return fmt.Errorf("node suffix_start_slave_cmd mismatch: expected %s, got %s", expected.SuffixStartSlaveCmd, actual.SuffixStartSlaveCmd)
	}

	if actual.PrefixStartSlaveCmd != expected.PrefixStartSlaveCmd {
		return fmt.Errorf("node prefix_start_slave_cmd mismatch: expected %s, got %s", expected.PrefixStartSlaveCmd, actual.PrefixStartSlaveCmd)
	}

	return nil
}

// TestNodeWithEnvironmentVariables tests node creation with environment variables
func TestNodeWithEnvironmentVariables(t *testing.T) {
	resourceConfigStep1 := `
resource "jenkins_node" "test" {
	name = "test_node_env"
	executors = 2
	description = "Node with environment variables"
	remote_fs = "/var/jenkins"
	labels = ["env-test"]
	environment_variables = {
		"JAVA_HOME" = "/usr/lib/jvm/java-11"
		"PATH" = "/usr/local/bin:/usr/bin"
		"MY_VAR" = "my_value"
	}
	launcher_configuration = {
		type = "jnlp"	
	}
}
	`

	resourceConfigStep2 := `
resource "jenkins_node" "test" {
	name = "test_node_env"
	executors = 2
	description = "Node with updated environment variables"
	remote_fs = "/var/jenkins"
	labels = ["env-test"]
	environment_variables = {
		"JAVA_HOME" = "/usr/lib/jvm/java-17"
		"NEW_VAR" = "new_value"
	}
	launcher_configuration = {
		type = "jnlp"	
	}
}
	`

	mergedConfig1 := providerConfig + "\n" + resourceConfigStep1
	mergedConfig2 := providerConfig + "\n" + resourceConfigStep2
	pd := getProviderData(testContainer)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: templateConfig(t, mergedConfig1, pd),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(testNodeResourceName, "name", "test_node_env"),
					resource.TestCheckResourceAttr(testNodeResourceName, "executors", "2"),
					resource.TestCheckResourceAttr(testNodeResourceName, "environment_variables.JAVA_HOME", "/usr/lib/jvm/java-11"),
					resource.TestCheckResourceAttr(testNodeResourceName, "environment_variables.PATH", "/usr/local/bin:/usr/bin"),
					resource.TestCheckResourceAttr(testNodeResourceName, "environment_variables.MY_VAR", "my_value"),
					verifyNodeEnvironmentVariables("test_node_env", map[string]string{
						"JAVA_HOME": "/usr/lib/jvm/java-11",
						"PATH":      "/usr/local/bin:/usr/bin",
						"MY_VAR":    "my_value",
					}),
				),
			},
			{
				Config: templateConfig(t, mergedConfig2, pd),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(testNodeResourceName, "environment_variables.JAVA_HOME", "/usr/lib/jvm/java-17"),
					resource.TestCheckResourceAttr(testNodeResourceName, "environment_variables.NEW_VAR", "new_value"),
					resource.TestCheckNoResourceAttr(testNodeResourceName, "environment_variables.PATH"),
					resource.TestCheckNoResourceAttr(testNodeResourceName, "environment_variables.MY_VAR"),
					verifyNodeEnvironmentVariables("test_node_env", map[string]string{
						"JAVA_HOME": "/usr/lib/jvm/java-17",
						"NEW_VAR":   "new_value",
					}),
				),
			},
		},
	})
}

// TestNodeWithToolLocations tests node creation with tool locations
func TestNodeWithToolLocations(t *testing.T) {
	resourceConfig := `
resource "jenkins_node" "test" {
	name = "test_node_tools"
	executors = 1
	description = "Node with tool locations"
	remote_fs = "/var/jenkins"
	labels = ["tools-test"]
	tool_locations = {
		"hudson.plugins.git.GitTool$DescriptorImpl:Default" = "/usr/bin/git"
		"hudson.model.JDK:JDK11" = "/usr/lib/jvm/java-11"
	}
	launcher_configuration = {
		type = "jnlp"	
	}
}
	`

	mergedConfig := providerConfig + "\n" + resourceConfig
	pd := getProviderData(testContainer)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: templateConfig(t, mergedConfig, pd),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(testNodeResourceName, "name", "test_node_tools"),
					resource.TestCheckResourceAttr(testNodeResourceName, "tool_locations.hudson.plugins.git.GitTool$DescriptorImpl:Default", "/usr/bin/git"),
					resource.TestCheckResourceAttr(testNodeResourceName, "tool_locations.hudson.model.JDK:JDK11", "/usr/lib/jvm/java-11"),
					verifyNodeToolLocations("test_node_tools"),
				),
			},
		},
	})
}

// TestNodeWithDiskSpaceMonitor tests node creation with disk space monitoring
func TestNodeWithDiskSpaceMonitor(t *testing.T) {
	resourceConfigStep1 := `
resource "jenkins_node" "test" {
	name = "test_node_disk"
	executors = 1
	description = "Node with disk space monitor"
	remote_fs = "/var/jenkins"
	labels = ["disk-test"]
	free_disk_space_threshold = "1GiB"
	launcher_configuration = {
		type = "jnlp"	
	}
}
	`

	resourceConfigStep2 := `
resource "jenkins_node" "test" {
	name = "test_node_disk"
	executors = 1
	description = "Node with updated disk space monitor"
	remote_fs = "/var/jenkins"
	labels = ["disk-test"]
	free_disk_space_threshold = "3GiB"
	free_temp_space_threshold = "2GiB"
	launcher_configuration = {
		type = "jnlp"	
	}
}
	`

	mergedConfig1 := providerConfig + "\n" + resourceConfigStep1
	mergedConfig2 := providerConfig + "\n" + resourceConfigStep2
	pd := getProviderData(testContainer)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: templateConfig(t, mergedConfig1, pd),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(testNodeResourceName, "name", "test_node_disk"),
					resource.TestCheckResourceAttr(testNodeResourceName, "free_disk_space_threshold", "1GiB"),
					verifyNodeDiskSpaceMonitor("test_node_disk", "1GiB"),
				),
			},
			{
				Config: templateConfig(t, mergedConfig2, pd),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(testNodeResourceName, "free_disk_space_threshold", "3GiB"),
					resource.TestCheckResourceAttr(testNodeResourceName, "free_temp_space_threshold", "2GiB"),
					verifyNodeDiskSpaceThresholds("test_node_disk", "3GiB", "2GiB"),
				),
			},
		},
	})
}

// verifyNodeEnvironmentVariables verifies environment variables are set correctly in Jenkins
func verifyNodeEnvironmentVariables(nodeName string, expectedVars map[string]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		node, err := testContainer.Jenkins.GetNode(ctx, nodeName)
		if err != nil {
			return fmt.Errorf("failed to get node: %w", err)
		}

		config, err := node.GetSlaveConfig(ctx)
		if err != nil {
			return fmt.Errorf("failed to get node config: %w", err)
		}

		if config.NodeProperties == nil || len(config.NodeProperties.Properties) == 0 {
			return fmt.Errorf("node has no properties")
		}

		var envProp *gojenkins.EnvironmentVariablesNodeProperty
		for _, prop := range config.NodeProperties.Properties {
			if p, ok := prop.(*gojenkins.EnvironmentVariablesNodeProperty); ok {
				envProp = p
				break
			}
		}

		if envProp == nil {
			return fmt.Errorf("no environment variables property found")
		}

		actualVars := make(map[string]string)
		for _, env := range envProp.EnvVars.Tree {
			actualVars[env.Key] = env.Value
		}

		for key, expectedValue := range expectedVars {
			if actualValue, ok := actualVars[key]; !ok {
				return fmt.Errorf("environment variable %s not found", key)
			} else if actualValue != expectedValue {
				return fmt.Errorf("environment variable %s has value %s, expected %s", key, actualValue, expectedValue)
			}
		}

		return nil
	}
}

// verifyNodeToolLocations verifies tool locations are set correctly in Jenkins
func verifyNodeToolLocations(nodeName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		node, err := testContainer.Jenkins.GetNode(ctx, nodeName)
		if err != nil {
			return fmt.Errorf("failed to get node: %w", err)
		}

		config, err := node.GetSlaveConfig(ctx)
		if err != nil {
			return fmt.Errorf("failed to get node config: %w", err)
		}

		if config.NodeProperties == nil || len(config.NodeProperties.Properties) == 0 {
			return fmt.Errorf("node has no properties")
		}

		var toolProp *gojenkins.ToolLocationNodeProperty
		for _, prop := range config.NodeProperties.Properties {
			if p, ok := prop.(*gojenkins.ToolLocationNodeProperty); ok {
				toolProp = p
				break
			}
		}

		if toolProp == nil {
			return fmt.Errorf("no tool location property found")
		}

		if len(toolProp.Locations) == 0 {
			return fmt.Errorf("no tool locations configured")
		}

		return nil
	}
}

// verifyNodeDiskSpaceMonitor verifies disk space monitor is set correctly in Jenkins
func verifyNodeDiskSpaceMonitor(nodeName string, expectedThreshold string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		node, err := testContainer.Jenkins.GetNode(ctx, nodeName)
		if err != nil {
			return fmt.Errorf("failed to get node: %w", err)
		}

		config, err := node.GetSlaveConfig(ctx)
		if err != nil {
			return fmt.Errorf("failed to get node config: %w", err)
		}

		if config.NodeProperties == nil || len(config.NodeProperties.Properties) == 0 {
			return fmt.Errorf("node has no properties")
		}

		var diskProp *gojenkins.DiskSpaceMonitorNodeProperty
		for _, prop := range config.NodeProperties.Properties {
			if p, ok := prop.(*gojenkins.DiskSpaceMonitorNodeProperty); ok {
				diskProp = p
				break
			}
		}

		if diskProp == nil {
			return fmt.Errorf("no disk space monitor property found")
		}

		if diskProp.FreeDiskSpaceThreshold != expectedThreshold {
			return fmt.Errorf("disk space threshold is %s, expected %s", diskProp.FreeDiskSpaceThreshold, expectedThreshold)
		}

		return nil
	}
}

// TestNodeWithAllDiskSpaceThresholds tests node creation with all 4 disk space threshold fields
func TestNodeWithAllDiskSpaceThresholds(t *testing.T) {
	resourceConfig := `
resource "jenkins_node" "test" {
	name = "test_node_all_thresholds"
	executors = 1
	description = "Node with all disk space thresholds"
	remote_fs = "/var/jenkins"
	labels = ["disk-all-test"]
	free_disk_space_threshold = "1GiB"
	free_temp_space_threshold = "500MiB"
	free_disk_space_warning_threshold = "800MiB"
	free_temp_space_warning_threshold = "400MiB"
	launcher_configuration = {
		type = "jnlp"	
	}
}
	`

	mergedConfig := providerConfig + "\n" + resourceConfig
	pd := getProviderData(testContainer)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: templateConfig(t, mergedConfig, pd),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(testNodeResourceName, "name", "test_node_all_thresholds"),
					resource.TestCheckResourceAttr(testNodeResourceName, "free_disk_space_threshold", "1GiB"),
					resource.TestCheckResourceAttr(testNodeResourceName, "free_temp_space_threshold", "500MiB"),
					resource.TestCheckResourceAttr(testNodeResourceName, "free_disk_space_warning_threshold", "800MiB"),
					resource.TestCheckResourceAttr(testNodeResourceName, "free_temp_space_warning_threshold", "400MiB"),
					verifyNodeAllDiskSpaceThresholds("test_node_all_thresholds", "1GiB", "500MiB", "800MiB", "400MiB"),
				),
			},
		},
	})
}

// TestNodeWithDeferredWipeout tests node creation with workspace cleanup (deferred wipeout)
func TestNodeWithDeferredWipeout(t *testing.T) {
	resourceConfig := `
resource "jenkins_node" "test" {
	name = "test_node_wipeout"
	executors = 1
	description = "Node with deferred wipeout disabled"
	remote_fs = "/var/jenkins"
	labels = ["wipeout-test"]
	disable_deferred_wipeout = true
	launcher_configuration = {
		type = "jnlp"	
	}
}
	`

	mergedConfig := providerConfig + "\n" + resourceConfig
	pd := getProviderData(testContainer)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: templateConfig(t, mergedConfig, pd),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(testNodeResourceName, "name", "test_node_wipeout"),
					resource.TestCheckResourceAttr(testNodeResourceName, "disable_deferred_wipeout", "true"),
					verifyNodeWorkspaceCleanup("test_node_wipeout"),
				),
			},
		},
	})
}

// TestNodeWithAllProperties tests node creation with all node properties combined
func TestNodeWithAllProperties(t *testing.T) {
	resourceConfig := `
resource "jenkins_node" "test" {
	name = "test_node_all_properties"
	executors = 2
	description = "Node with all properties combined"
	remote_fs = "/var/jenkins"
	labels = ["all-properties-test"]
	environment_variables = {
		"TEST_VAR" = "test_value"
	}
	tool_locations = {
		"hudson.plugins.git.GitTool$DescriptorImpl:Default" = "/usr/bin/git"
	}
	free_disk_space_threshold = "2GiB"
	free_temp_space_threshold = "1GiB"
	free_disk_space_warning_threshold = "1500MiB"
	free_temp_space_warning_threshold = "800MiB"
	disable_deferred_wipeout = true
	launcher_configuration = {
		type = "jnlp"	
	}
}
	`

	mergedConfig := providerConfig + "\n" + resourceConfig
	pd := getProviderData(testContainer)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: templateConfig(t, mergedConfig, pd),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(testNodeResourceName, "name", "test_node_all_properties"),
					resource.TestCheckResourceAttr(testNodeResourceName, "executors", "2"),
					resource.TestCheckResourceAttr(testNodeResourceName, "environment_variables.TEST_VAR", "test_value"),
					resource.TestCheckResourceAttr(testNodeResourceName, "tool_locations.hudson.plugins.git.GitTool$DescriptorImpl:Default", "/usr/bin/git"),
					resource.TestCheckResourceAttr(testNodeResourceName, "free_disk_space_threshold", "2GiB"),
					resource.TestCheckResourceAttr(testNodeResourceName, "free_temp_space_threshold", "1GiB"),
					resource.TestCheckResourceAttr(testNodeResourceName, "free_disk_space_warning_threshold", "1500MiB"),
					resource.TestCheckResourceAttr(testNodeResourceName, "free_temp_space_warning_threshold", "800MiB"),
					resource.TestCheckResourceAttr(testNodeResourceName, "disable_deferred_wipeout", "true"),
					verifyNodeAllDiskSpaceThresholds("test_node_all_properties", "2GiB", "1GiB", "1500MiB", "800MiB"),
					verifyNodeWorkspaceCleanup("test_node_all_properties"),
				),
			},
			{
				Config: templateConfig(t, providerConfig+`
resource "jenkins_node" "test" {
	name = "test_node_all_properties"
	executors = 2
	description = "Node with properties removed"
	remote_fs = "/var/jenkins"
	labels = ["all-properties-test"]
	launcher_configuration = {
		type = "jnlp"	
	}
}
`, pd),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr(testNodeResourceName, "environment_variables.TEST_VAR"),
					resource.TestCheckNoResourceAttr(testNodeResourceName, "tool_locations.hudson.plugins.git.GitTool$DescriptorImpl:Default"),
					resource.TestCheckNoResourceAttr(testNodeResourceName, "free_disk_space_threshold"),
					resource.TestCheckNoResourceAttr(testNodeResourceName, "disable_deferred_wipeout"),
					verifyNodeHasNoProperties("test_node_all_properties"),
				),
			},
		},
	})
}

// verifyNodeHasNoProperties verifies that a node has no properties set
func verifyNodeHasNoProperties(nodeName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		node, err := testContainer.Jenkins.GetNode(ctx, nodeName)
		if err != nil {
			return fmt.Errorf("failed to get node: %w", err)
		}

		config, err := node.GetSlaveConfig(ctx)
		if err != nil {
			return fmt.Errorf("failed to get node config: %w", err)
		}

		if config.NodeProperties != nil && len(config.NodeProperties.Properties) > 0 {
			return fmt.Errorf("expected no properties, but found %d properties", len(config.NodeProperties.Properties))
		}

		return nil
	}
}

// verifyNodeAllDiskSpaceThresholds verifies all 4 disk space thresholds are set correctly in Jenkins
func verifyNodeAllDiskSpaceThresholds(nodeName, freeDisk, freeTemp, freeDiskWarning, freeTempWarning string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		node, err := testContainer.Jenkins.GetNode(ctx, nodeName)
		if err != nil {
			return fmt.Errorf("failed to get node: %w", err)
		}

		config, err := node.GetSlaveConfig(ctx)
		if err != nil {
			return fmt.Errorf("failed to get node config: %w", err)
		}

		if config.NodeProperties == nil || len(config.NodeProperties.Properties) == 0 {
			return fmt.Errorf("node has no properties")
		}

		var diskProp *gojenkins.DiskSpaceMonitorNodeProperty
		for _, prop := range config.NodeProperties.Properties {
			if p, ok := prop.(*gojenkins.DiskSpaceMonitorNodeProperty); ok {
				diskProp = p
				break
			}
		}

		if diskProp == nil {
			return fmt.Errorf("no disk space monitor property found")
		}

		if diskProp.FreeDiskSpaceThreshold != freeDisk {
			return fmt.Errorf("free disk space threshold is %s, expected %s", diskProp.FreeDiskSpaceThreshold, freeDisk)
		}

		if diskProp.FreeTempSpaceThreshold != freeTemp {
			return fmt.Errorf("free temp space threshold is %s, expected %s", diskProp.FreeTempSpaceThreshold, freeTemp)
		}

		if diskProp.FreeDiskSpaceWarningThreshold != freeDiskWarning {
			return fmt.Errorf("free disk space warning threshold is %s, expected %s", diskProp.FreeDiskSpaceWarningThreshold, freeDiskWarning)
		}

		if diskProp.FreeTempSpaceWarningThreshold != freeTempWarning {
			return fmt.Errorf("free temp space warning threshold is %s, expected %s", diskProp.FreeTempSpaceWarningThreshold, freeTempWarning)
		}

		return nil
	}
}

// verifyNodeDiskSpaceThresholds verifies the basic disk space thresholds (no warning thresholds)
func verifyNodeDiskSpaceThresholds(nodeName, freeDisk, freeTemp string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		node, err := testContainer.Jenkins.GetNode(ctx, nodeName)
		if err != nil {
			return fmt.Errorf("failed to get node: %w", err)
		}

		config, err := node.GetSlaveConfig(ctx)
		if err != nil {
			return fmt.Errorf("failed to get node config: %w", err)
		}

		if config.NodeProperties == nil || len(config.NodeProperties.Properties) == 0 {
			return fmt.Errorf("node has no properties")
		}

		var diskProp *gojenkins.DiskSpaceMonitorNodeProperty
		for _, prop := range config.NodeProperties.Properties {
			if p, ok := prop.(*gojenkins.DiskSpaceMonitorNodeProperty); ok {
				diskProp = p
				break
			}
		}

		if diskProp == nil {
			return fmt.Errorf("no disk space monitor property found")
		}

		if diskProp.FreeDiskSpaceThreshold != freeDisk {
			return fmt.Errorf("free disk space threshold is %s, expected %s", diskProp.FreeDiskSpaceThreshold, freeDisk)
		}

		if diskProp.FreeTempSpaceThreshold != freeTemp {
			return fmt.Errorf("free temp space threshold is %s, expected %s", diskProp.FreeTempSpaceThreshold, freeTemp)
		}

		return nil
	}
}

// verifyNodeWorkspaceCleanup verifies workspace cleanup property is set correctly in Jenkins
func verifyNodeWorkspaceCleanup(nodeName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		ctx := context.Background()
		node, err := testContainer.Jenkins.GetNode(ctx, nodeName)
		if err != nil {
			return fmt.Errorf("failed to get node: %w", err)
		}

		config, err := node.GetSlaveConfig(ctx)
		if err != nil {
			return fmt.Errorf("failed to get node config: %w", err)
		}

		if config.NodeProperties == nil || len(config.NodeProperties.Properties) == 0 {
			return fmt.Errorf("node has no properties")
		}

		var workspaceProp *gojenkins.WorkspaceCleanupNodeProperty
		for _, prop := range config.NodeProperties.Properties {
			if p, ok := prop.(*gojenkins.WorkspaceCleanupNodeProperty); ok {
				workspaceProp = p
				break
			}
		}

		if workspaceProp == nil {
			return fmt.Errorf("no workspace cleanup property found")
		}

		return nil
	}
}

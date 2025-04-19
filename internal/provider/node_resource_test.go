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

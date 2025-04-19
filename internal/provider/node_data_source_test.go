// The provider package implements a Jenkins provider for Terraform
//
// node_data_source_test.go implements a set of tests to validate the data source
// matches the terraform resource configuration.
package provider

import (
	"context"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/aidanleuck/gojenkins"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/require"
)

// Name of the node resource and data source.
const (
	testNodeResourceName = "jenkins_node.test"
	testDataSourceName   = "data.jenkins_node.test_data"
)

var (
	nodeDataSourceConfig = `
	provider "jenkins" {
		url = "{{ .Server }}"
		username = "{{ .Username }}"
		password = "{{ .Password }}"
	}

	data "jenkins_node" "test_data" {
		name = "{{ .Name }}"
	}
`

	testContainer *gojenkins.ContainerizedTest
)

// Test data source basic, checks the data source contains all common
// fields between node types
func TestAccNodeDataSourceJNLP(t *testing.T) {
	p := getProviderData(testContainer)

	type testData struct {
		Name string
		providerData
	}

	d := &testData{
		Name:         "test-node",
		providerData: *p,
	}

	executors := 1
	remoteFs := "C:\\filesystem"
	description := "test node"
	labels := "hello world"

	type tc struct {
		name     string
		launcher *gojenkins.JNLPLauncher
	}

	testCases := []tc{
		{
			name:     "default",
			launcher: gojenkins.DefaultJNLPLauncher(),
		},
		{
			name: "websocket",
			launcher: gojenkins.NewJNLPLauncher(true, &gojenkins.WorkDirSettings{
				Disabled:               true,
				InternalDir:            "test",
				FailIfWorkDirIsMissing: true,
			}),
		},
		{
			name: "websocket with workdir",
			launcher: gojenkins.NewJNLPLauncher(true, &gojenkins.WorkDirSettings{
				Disabled:               false,
				InternalDir:            "test",
				FailIfWorkDirIsMissing: false,
			}),
		},
		{
			name: "no websocket",
			launcher: gojenkins.NewJNLPLauncher(false, &gojenkins.WorkDirSettings{
				Disabled:               true,
				InternalDir:            "test",
				FailIfWorkDirIsMissing: true,
			}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			n, err := testContainer.Jenkins.CreateNode(context.Background(), d.Name, executors, description, remoteFs, labels, tc.launcher)
			require.NoError(t, err)
			defer n.Delete(context.Background())

			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config: templateConfig(t, nodeDataSourceConfig, d),
						Check: resource.ComposeTestCheckFunc(
							resource.TestCheckResourceAttr(testDataSourceName, "launcher_configuration.type", "jnlp"),
							resource.TestCheckResourceAttr(testDataSourceName, "launcher_configuration.jnlp_options.workdir_disabled", strconv.FormatBool(tc.launcher.WorkDirSettings.Disabled)),
							resource.TestCheckResourceAttr(testDataSourceName, "launcher_configuration.jnlp_options.remoting_dir", tc.launcher.WorkDirSettings.InternalDir),
							resource.TestCheckResourceAttr(testDataSourceName, "launcher_configuration.jnlp_options.fail_if_workdir_missing", strconv.FormatBool(tc.launcher.WorkDirSettings.FailIfWorkDirIsMissing)),
							resource.TestCheckResourceAttr(testDataSourceName, "launcher_configuration.jnlp_options.use_web_socket", strconv.FormatBool(tc.launcher.WebSocket)),
							resource.TestCheckResourceAttrSet(testDataSourceName, "jnlp_secret"),
							resource.TestCheckNoResourceAttr(testDataSourceName, "launcher_configuration.ssh_options"),
						),
					},
				},
			})
		})
	}
}

func TestAccNodeDataSourceSSH(t *testing.T) {
	p := getProviderData(testContainer)

	type testData struct {
		Name string
		providerData
	}

	d := &testData{
		Name:         "test-node",
		providerData: *p,
	}

	executors := 1
	remoteFs := "C:\\filesystem"
	description := "test node"
	labels := "hello world"

	type tc struct {
		name     string
		launcher *gojenkins.SSHLauncher
	}

	testCases := []tc{
		{
			name:     "default",
			launcher: gojenkins.DefaultSSHLauncher(),
		},
		{
			name:     "custom-options",
			launcher: gojenkins.NewSSHLauncher("localhost", 55, "test-cred", 55, 20, 20, "test-jvm", "test-java", "test-prefix", "test-suffix"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			n, err := testContainer.Jenkins.CreateNode(context.Background(), d.Name, executors, description, remoteFs, labels, tc.launcher)
			require.NoError(t, err)
			defer n.Delete(context.Background())

			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config: templateConfig(t, nodeDataSourceConfig, d),
						Check: resource.ComposeTestCheckFunc(
							resource.TestCheckResourceAttr(testDataSourceName, "launcher_configuration.type", "ssh"),
							resource.TestCheckResourceAttr(testDataSourceName, "launcher_configuration.ssh_options.host", tc.launcher.Host),
							resource.TestCheckResourceAttr(testDataSourceName, "launcher_configuration.ssh_options.port", strconv.Itoa(tc.launcher.Port)),
							resource.TestCheckResourceAttr(testDataSourceName, "launcher_configuration.ssh_options.credentials_id", tc.launcher.CredentialsId),
							resource.TestCheckResourceAttr(testDataSourceName, "launcher_configuration.ssh_options.retry_wait_time", strconv.Itoa(tc.launcher.RetryWaitTime)),
							resource.TestCheckResourceAttr(testDataSourceName, "launcher_configuration.ssh_options.launch_timeout_seconds", strconv.Itoa(tc.launcher.LaunchTimeoutSeconds)),
							resource.TestCheckResourceAttr(testDataSourceName, "launcher_configuration.ssh_options.jvm_options", tc.launcher.JvmOptions),
							resource.TestCheckResourceAttr(testDataSourceName, "launcher_configuration.ssh_options.java_path", tc.launcher.JavaPath),
							resource.TestCheckResourceAttr(testDataSourceName, "launcher_configuration.ssh_options.prefix_start_slave_cmd", tc.launcher.PrefixStartSlaveCmd),
							resource.TestCheckResourceAttr(testDataSourceName, "launcher_configuration.ssh_options.suffix_start_slave_cmd", tc.launcher.SuffixStartSlaveCmd),
							resource.TestCheckResourceAttr(testDataSourceName, "launcher_configuration.ssh_options.launch_timeout_seconds", strconv.Itoa(tc.launcher.LaunchTimeoutSeconds)),
							resource.TestCheckNoResourceAttr(testDataSourceName, "jnlp_secret"),
							resource.TestCheckNoResourceAttr(testDataSourceName, "launcher_configuration.jnlp_options"),
						),
					},
				},
			})
		})
	}
}

func TestBasicNodeDataSource(t *testing.T) {
	p := getProviderData(testContainer)

	type testData struct {
		Name string
		providerData
	}

	d := &testData{
		Name:         "test-node",
		providerData: *p,
	}
	type tc struct {
		name        string
		executors   int
		description string
		remoteFs    string
		labels      string
	}

	testCases := []tc{
		{
			name:        "normal-node",
			executors:   1,
			description: "normal node",
			remoteFs:    "C:\\normal-fs",
			labels:      "normal-label",
		},
		{
			name:        "multi-executor-node",
			executors:   5,
			description: "multi-executor node",
			remoteFs:    "C:\\multi-fs",
			labels:      "multi-label",
		},
		{
			name:        "multi-label-node",
			executors:   1,
			description: "multi-label node",
			remoteFs:    "C:\\multi-label-fs",
			labels:      "label1 label2 label3",
		},
		{
			name:        "empty-label-node",
			executors:   1,
			description: "empty-label node",
			remoteFs:    "C:\\empty-label-fs",
			labels:      "",
		},
		{
			name:        "empty-description-node",
			executors:   1,
			description: "",
			remoteFs:    "C:\\empty-description-fs",
			labels:      "label1 label2",
		},
		{
			name:        "empty-remote-fs-node",
			executors:   1,
			description: "empty-remote-fs node",
			remoteFs:    "",
			labels:      "label1 label2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			n, err := testContainer.Jenkins.CreateNode(context.Background(), d.Name, tc.executors, tc.description, tc.remoteFs, tc.labels, gojenkins.DefaultJNLPLauncher())
			require.NoError(t, err)
			defer n.Delete(context.Background())

			labelCases := getLabelTestFunction(tc.labels)
			otherFuncs := []resource.TestCheckFunc{
				resource.TestCheckResourceAttr(testDataSourceName, "name", d.Name),
				resource.TestCheckResourceAttr(testDataSourceName, "executors", strconv.Itoa(tc.executors)),
				resource.TestCheckResourceAttr(testDataSourceName, "description", tc.description),
				resource.TestCheckResourceAttr(testDataSourceName, "remote_fs", tc.remoteFs),
				resource.TestCheckResourceAttr(testDataSourceName, "launcher_configuration.type", "jnlp"),
			}

			allCases := make([]resource.TestCheckFunc, 0, len(labelCases)+len(otherFuncs))
			allCases = append(allCases, otherFuncs...)
			allCases = append(allCases, labelCases...)

			resource.Test(t, resource.TestCase{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{
					{
						Config: templateConfig(t, nodeDataSourceConfig, d),
						Check: resource.ComposeTestCheckFunc(
							allCases...,
						),
					},
				},
			})
		})
	}
}

func getLabelTestFunction(label string) []resource.TestCheckFunc {
	// If an empty label is passed, we check that the labels list is empty
	// in the data source
	if label == "" {
		return []resource.TestCheckFunc{
			resource.TestCheckResourceAttr(testDataSourceName, "labels.#", "0"),
		}
	}

	// Split the label string into a list of labels
	// and create a list of test functions to check each label
	// against the data source
	labels := strings.Split(label, " ")
	testFuncs := make([]resource.TestCheckFunc, len(labels))
	if len(labels) == 0 {
		return testFuncs
	}

	for i, label := range labels {
		testFuncs[i] = resource.TestCheckResourceAttr(testDataSourceName, "labels."+strconv.Itoa(i), label)
	}

	return testFuncs
}

func TestNoNodeError(t *testing.T) {
	type testData struct {
		Name string
		providerData
	}

	p := getProviderData(testContainer)
	d := &testData{
		Name:         "test-node",
		providerData: *p,
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      templateConfig(t, nodeDataSourceConfig, d),
				ExpectError: regexp.MustCompile(".+failed to find jenkins node.+"),
			},
		},
	})
}

func TestMain(t *testing.M) {
	ctx := context.Background()
	c, err := gojenkins.Setup(ctx, "test", "test")
	if err != nil {
		panic(err)
	}

	defer c.CleanupFunc()
	testContainer = c

	os.Exit(t.Run())
}

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// The provider package implements a Jenkins provider for Terraform
//
// folder_resource_test.go implements acceptance tests for the folder resource
// and data source. They require the Jenkins test container.
package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const (
	testFolderResourceName   = "jenkins_folder.test"
	testFolderDataSourceName = "data.jenkins_folder.test_data"
)

// checkFolderExists verifies the folder exists (or not) in Jenkins.
func checkFolderExists(parent, name string, shouldExist bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		_, found, err := getFolder(context.Background(), testContainer.Jenkins, parent, name)
		if err != nil {
			return err
		}
		if found != shouldExist {
			return fmt.Errorf("folder %s exists = %t, expected %t", buildFolderID(parent, name), found, shouldExist)
		}
		return nil
	}
}

func checkFolderDestroyed(parent, name string) resource.TestCheckFunc {
	return checkFolderExists(parent, name, false)
}

func TestAccFolderResourceBasic(t *testing.T) {
	pd := getProviderData(testContainer)

	config := func(description string) string {
		return templateConfig(t, providerConfig+fmt.Sprintf(`
resource "jenkins_folder" "test" {
	name = "basic-folder"
	description = "%s"
}
`, description), pd)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkFolderDestroyed("", "basic-folder"),
		Steps: []resource.TestStep{
			{
				Config: config("first"),
				Check: resource.ComposeTestCheckFunc(
					checkFolderExists("", "basic-folder", true),
					resource.TestCheckResourceAttr(testFolderResourceName, "id", "basic-folder"),
					resource.TestCheckResourceAttr(testFolderResourceName, "name", "basic-folder"),
					resource.TestCheckResourceAttr(testFolderResourceName, "description", "first"),
					resource.TestCheckNoResourceAttr(testFolderResourceName, "folder"),
					resource.TestCheckResourceAttrSet(testFolderResourceName, "url"),
				),
			},
			// In place update of the description.
			{
				Config: config("second"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(testFolderResourceName, "description", "second"),
				),
			},
			// Drift: the description is changed outside of Terraform and is restored.
			{
				PreConfig: driftFolderDescription(t, "", "basic-folder", "drifted"),
				Config:    config("second"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(testFolderResourceName, "description", "second"),
				),
			},
			// Import by path.
			{
				ResourceName:      testFolderResourceName,
				ImportState:       true,
				ImportStateId:     "basic-folder",
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFolderResourceNoDescription(t *testing.T) {
	pd := getProviderData(testContainer)

	config := templateConfig(t, providerConfig+`
resource "jenkins_folder" "test" {
	name = "no-description-folder"
}
`, pd)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkFolderDestroyed("", "no-description-folder"),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					checkFolderExists("", "no-description-folder", true),
					resource.TestCheckNoResourceAttr(testFolderResourceName, "description"),
				),
			},
			// Drift: a description added outside of Terraform is removed again.
			{
				PreConfig: driftFolderDescription(t, "", "no-description-folder", "drifted"),
				Config:    config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr(testFolderResourceName, "description"),
				),
			},
		},
	})
}

func TestAccFolderResourceNested(t *testing.T) {
	pd := getProviderData(testContainer)

	config := func(childName string) string {
		return templateConfig(t, providerConfig+fmt.Sprintf(`
resource "jenkins_folder" "parent" {
	name = "nested-parent"
}

resource "jenkins_folder" "test" {
	name = "%s"
	folder = jenkins_folder.parent.id
}

resource "jenkins_folder" "grandchild" {
	name = "grandchild"
	folder = jenkins_folder.test.id
}
`, childName), pd)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkFolderDestroyed("", "nested-parent"),
		Steps: []resource.TestStep{
			{
				Config: config("child"),
				Check: resource.ComposeTestCheckFunc(
					checkFolderExists("nested-parent", "child", true),
					checkFolderExists("nested-parent/child", "grandchild", true),
					resource.TestCheckResourceAttr(testFolderResourceName, "id", "nested-parent/child"),
					resource.TestCheckResourceAttr(testFolderResourceName, "folder", "nested-parent"),
					resource.TestCheckResourceAttr("jenkins_folder.grandchild", "id", "nested-parent/child/grandchild"),
				),
			},
			// Changing the name replaces the folder.
			{
				Config: config("renamed"),
				Check: resource.ComposeTestCheckFunc(
					checkFolderExists("nested-parent", "child", false),
					checkFolderExists("nested-parent", "renamed", true),
					resource.TestCheckResourceAttr(testFolderResourceName, "id", "nested-parent/renamed"),
				),
			},
			{
				ResourceName:      "jenkins_folder.grandchild",
				ImportState:       true,
				ImportStateId:     "nested-parent/renamed/grandchild",
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFolderResourceInvalidName(t *testing.T) {
	pd := getProviderData(testContainer)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: templateConfig(t, providerConfig+`
resource "jenkins_folder" "test" {
	name = "a/b"
}
`, pd),
				ExpectError: regexp.MustCompile(`must not be empty or contain`),
			},
		},
	})
}

func TestAccFolderDataSource(t *testing.T) {
	pd := getProviderData(testContainer)

	config := templateConfig(t, providerConfig+`
resource "jenkins_folder" "parent" {
	name = "ds-parent"
}

resource "jenkins_folder" "test" {
	name = "ds-child"
	folder = jenkins_folder.parent.id
	description = "data source folder"
}

data "jenkins_folder" "test_data" {
	name = jenkins_folder.test.name
	folder = jenkins_folder.test.folder
}
`, pd)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkFolderDestroyed("", "ds-parent"),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(testFolderDataSourceName, "id", "ds-parent/ds-child"),
					resource.TestCheckResourceAttr(testFolderDataSourceName, "description", "data source folder"),
					resource.TestCheckResourceAttrPair(testFolderDataSourceName, "url", testFolderResourceName, "url"),
				),
			},
		},
	})
}

func TestAccFolderDataSourceNotFound(t *testing.T) {
	pd := getProviderData(testContainer)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: templateConfig(t, providerConfig+`
data "jenkins_folder" "test_data" {
	name = "does-not-exist"
}
`, pd),
				ExpectError: regexp.MustCompile(`does not exist`),
			},
		},
	})
}

// driftFolderDescription changes the description directly in Jenkins to simulate drift.
func driftFolderDescription(t *testing.T, parent, name, description string) func() {
	return func() {
		ctx := context.Background()
		folder, found, err := getFolder(ctx, testContainer.Jenkins, parent, name)
		if err != nil || !found {
			t.Fatalf("failed to find folder %s: found=%t err=%v", buildFolderID(parent, name), found, err)
		}
		if err := setFolderDescription(ctx, testContainer.Jenkins, folder, description); err != nil {
			t.Fatalf("failed to drift folder description: %v", err)
		}
	}
}

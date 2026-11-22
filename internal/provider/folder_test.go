// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// folder_test.go implements acceptance tests for the folder resource and data source.
package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/require"
)

func TestFolderID(t *testing.T) {
	require.Equal(t, "a", folderID("", "a"))
	require.Equal(t, "a/b/c", folderID("a/b", "c"))

	parent, name := splitFolderID("a/b/c")
	require.Equal(t, "a/b", parent)
	require.Equal(t, "c", name)

	parent, name = splitFolderID("a")
	require.Equal(t, "", parent)
	require.Equal(t, "a", name)

	require.Nil(t, splitFolderPath(""))
	require.Equal(t, []string{"a", "b"}, splitFolderPath("a/b"))
}

type folderTestData struct {
	providerData
	Name        string
	Parent      string
	Description string
}

const folderResourceConfig = providerConfig + `
resource "jenkins_folder" "test" {
	name = "{{ .Name }}"
	{{- if .Parent }}
	parent = "{{ .Parent }}"
	{{- end }}
	{{- if .Description }}
	description = "{{ .Description }}"
	{{- end }}
}
`

func checkFolderDestroyed(id string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		parent, name := splitFolderID(id)
		if _, err := getFolder(context.Background(), testContainer.Jenkins, parent, name); err == nil {
			return fmt.Errorf("folder %s still exists", id)
		}
		return nil
	}
}

func TestAccFolderResourceCreateUpdate(t *testing.T) {
	d := folderTestData{providerData: *getProviderData(testContainer), Name: "tf-folder", Description: "first"}
	updated := d
	updated.Description = "second"
	renamed := updated
	renamed.Name = "tf-folder-renamed"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkFolderDestroyed("tf-folder-renamed"),
		Steps: []resource.TestStep{
			{
				Config: templateConfig(t, folderResourceConfig, d),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("jenkins_folder.test", "id", "tf-folder"),
					resource.TestCheckResourceAttr("jenkins_folder.test", "description", "first"),
				),
			},
			{
				// Description changes are applied in place.
				Config: templateConfig(t, folderResourceConfig, updated),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("jenkins_folder.test", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.TestCheckResourceAttr("jenkins_folder.test", "description", "second"),
			},
			{
				// Name changes force replacement.
				Config: templateConfig(t, folderResourceConfig, renamed),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("jenkins_folder.test", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: resource.TestCheckResourceAttr("jenkins_folder.test", "id", "tf-folder-renamed"),
			},
		},
	})
}

func TestAccFolderResourceNested(t *testing.T) {
	config := providerConfig + `
resource "jenkins_folder" "parent" {
	name = "tf-parent"
}

resource "jenkins_folder" "child" {
	name   = "tf-child"
	parent = jenkins_folder.parent.id
}

resource "jenkins_folder" "grandchild" {
	name   = "tf-grandchild"
	parent = jenkins_folder.child.id
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkFolderDestroyed("tf-parent"),
		Steps: []resource.TestStep{
			{
				Config: templateConfig(t, config, getProviderData(testContainer)),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("jenkins_folder.child", "id", "tf-parent/tf-child"),
					resource.TestCheckResourceAttr("jenkins_folder.grandchild", "id", "tf-parent/tf-child/tf-grandchild"),
				),
			},
			{
				ResourceName:      "jenkins_folder.grandchild",
				ImportState:       true,
				ImportStateId:     "tf-parent/tf-child/tf-grandchild",
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFolderResourceImport(t *testing.T) {
	d := folderTestData{providerData: *getProviderData(testContainer), Name: "tf-import", Description: "imported"}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             checkFolderDestroyed("tf-import"),
		Steps: []resource.TestStep{
			{Config: templateConfig(t, folderResourceConfig, d)},
			{
				ResourceName:      "jenkins_folder.test",
				ImportState:       true,
				ImportStateId:     "tf-import",
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccFolderResourceMissingParent(t *testing.T) {
	d := folderTestData{providerData: *getProviderData(testContainer), Name: "tf-orphan", Parent: "does-not-exist"}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      templateConfig(t, folderResourceConfig, d),
				ExpectError: regexp.MustCompile(`parent folder "does-not-exist" does not exist`),
			},
		},
	})
}

func TestAccFolderResourceDeletedOutOfBand(t *testing.T) {
	d := folderTestData{providerData: *getProviderData(testContainer), Name: "tf-oob"}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: templateConfig(t, folderResourceConfig, d)},
			{
				PreConfig: func() {
					f, err := getFolder(context.Background(), testContainer.Jenkins, "", "tf-oob")
					require.NoError(t, err)
					require.NoError(t, f.Delete(context.Background()))
				},
				// Refresh detects the deletion, so the plan wants to create the folder again.
				Config:             templateConfig(t, folderResourceConfig, d),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccFolderDataSource(t *testing.T) {
	config := providerConfig + `
data "jenkins_folder" "test" {
	name   = "tf-ds-child"
	parent = "tf-ds-parent"
}
`
	missing := providerConfig + `
data "jenkins_folder" "test" {
	name = "tf-ds-missing"
}
`
	ctx := context.Background()
	_, err := testContainer.Jenkins.CreateFolder(ctx, "tf-ds-parent")
	require.NoError(t, err)
	child, err := testContainer.Jenkins.CreateFolder(ctx, "tf-ds-child", "tf-ds-parent")
	require.NoError(t, err)
	require.NoError(t, setFolderDescription(ctx, testContainer.Jenkins, child, "ds description"))
	defer func() {
		parent, err := getFolder(ctx, testContainer.Jenkins, "", "tf-ds-parent")
		if err == nil {
			_ = parent.Delete(ctx)
		}
	}()

	pd := getProviderData(testContainer)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: templateConfig(t, config, pd),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.jenkins_folder.test", "id", "tf-ds-parent/tf-ds-child"),
					resource.TestCheckResourceAttr("data.jenkins_folder.test", "description", "ds description"),
				),
			},
			{
				Config:      templateConfig(t, missing, pd),
				ExpectError: regexp.MustCompile(`folder tf-ds-missing was not found`),
			},
		},
	})
}

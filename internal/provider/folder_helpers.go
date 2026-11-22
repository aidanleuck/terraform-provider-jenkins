// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// The provider package implements a Jenkins provider for Terraform
//
// folder_helpers.go implements utility methods shared by the folder resource and
// data source for working with folder paths and the underlying gojenkins library.
package provider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/aidanleuck/gojenkins"
)

// folderID builds the full path of a folder from its parent path and name.
func folderID(parent, name string) string {
	if parent == "" {
		return name
	}

	return parent + "/" + name
}

// splitFolderID splits a full folder path into its parent path and name.
func splitFolderID(id string) (parent string, name string) {
	idx := strings.LastIndex(id, "/")
	if idx < 0 {
		return "", id
	}

	return id[:idx], id[idx+1:]
}

// splitFolderPath splits a folder path into its segments. An empty path has no segments.
func splitFolderPath(parent string) []string {
	if parent == "" {
		return nil
	}

	return strings.Split(parent, "/")
}

// getFolder retrieves the folder with the given name under parent. The returned error
// wraps gojenkins.ErrNotFound if the folder does not exist.
func getFolder(ctx context.Context, client *gojenkins.Jenkins, parent, name string) (*gojenkins.Folder, error) {
	return client.GetFolder(ctx, name, splitFolderPath(parent)...)
}

// ensureParentExists returns a descriptive error if the parent folder path does not exist.
func ensureParentExists(ctx context.Context, client *gojenkins.Jenkins, parent string) error {
	if parent == "" {
		return nil
	}

	grandParent, name := splitFolderID(parent)
	if _, err := getFolder(ctx, client, grandParent, name); err != nil {
		if errors.Is(err, gojenkins.ErrNotFound) {
			return fmt.Errorf("parent folder %q does not exist", parent)
		}

		return fmt.Errorf("failed to look up parent folder %q: %w", parent, err)
	}

	return nil
}

// setFolderDescription updates the description of a folder. The gojenkins library does not
// support this for folders, so the submitDescription endpoint is called directly.
func setFolderDescription(ctx context.Context, client *gojenkins.Jenkins, folder *gojenkins.Folder, description string) error {
	data := url.Values{}
	data.Set("description", description)

	resp, err := client.Requester.Post(ctx, folder.Base+"/submitDescription", bytes.NewBufferString(data.Encode()), nil, nil)
	if err != nil {
		return err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	return nil
}

var (
	// folderNameRegex matches names that do not contain a path separator.
	folderNameRegex = regexp.MustCompile(`^[^/]+$`)

	// folderParentRegex matches paths of folder names separated by '/', or an empty path.
	folderParentRegex = regexp.MustCompile(`^([^/]+(/[^/]+)*)?$`)
)

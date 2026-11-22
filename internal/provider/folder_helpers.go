// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// The provider package implements a Jenkins provider for Terraform
//
// folder_helpers.go contains helpers shared by the folder resource and data source.
package provider

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/aidanleuck/gojenkins"
)

var (
	// folderNameRegex matches a single path segment (no slashes).
	folderNameRegex = regexp.MustCompile(`^[^/]+$`)
	// folderPathRegex matches a slash separated path without leading or trailing slashes.
	folderPathRegex = regexp.MustCompile(`^[^/]+(/[^/]+)*$`)

	descriptionElementRegex = regexp.MustCompile(`(?s)<description>.*?</description>|<description\s*/>`)
	rootElementRegex        = regexp.MustCompile(`<[A-Za-z][^>]*>`)
)

// splitFolderPath splits a slash separated folder path into its segments.
// An empty path returns no segments.
func splitFolderPath(p string) []string {
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

// buildFolderID joins the parent folder path and name into the full folder path.
func buildFolderID(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "/" + name
}

// parseFolderID splits a full folder path into the parent folder path and the name.
func parseFolderID(id string) (parent string, name string, err error) {
	if !folderPathRegex.MatchString(id) {
		return "", "", fmt.Errorf("invalid folder path %q, expected the form `name` or `parent/name`", id)
	}
	if i := strings.LastIndex(id, "/"); i >= 0 {
		return id[:i], id[i+1:], nil
	}
	return "", id, nil
}

// getFolder retrieves a folder from Jenkins. The returned bool is false when the folder does not exist.
func getFolder(ctx context.Context, client *gojenkins.Jenkins, parent, name string) (*gojenkins.Folder, bool, error) {
	folder, err := client.GetFolder(ctx, name, splitFolderPath(parent)...)
	if err != nil {
		if errors.Is(err, gojenkins.ErrNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return folder, true, nil
}

// setFolderDescription updates only the description of a folder by rewriting its config.xml.
func setFolderDescription(ctx context.Context, client *gojenkins.Jenkins, folder *gojenkins.Folder, description string) error {
	var config string
	if _, err := client.Requester.GetXML(ctx, folder.Base+"/config.xml", &config, nil); err != nil {
		return fmt.Errorf("failed retrieving folder config: %w", err)
	}

	updated, err := replaceDescription(config, description)
	if err != nil {
		return err
	}

	resp, err := client.Requester.PostXML(ctx, folder.Base+"/config.xml", updated, nil, nil)
	if err != nil {
		return fmt.Errorf("failed updating folder config: %w", err)
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("failed updating folder config, status code: %d", resp.StatusCode)
	}

	return nil
}

// replaceDescription sets the description element of a folder config.xml,
// adding it as the first child of the root element if it is not present.
func replaceDescription(config, description string) (string, error) {
	var escaped bytes.Buffer
	if err := xml.EscapeText(&escaped, []byte(description)); err != nil {
		return "", err
	}
	element := "<description>" + escaped.String() + "</description>"

	if loc := descriptionElementRegex.FindStringIndex(config); loc != nil {
		return config[:loc[0]] + element + config[loc[1]:], nil
	}

	loc := rootElementRegex.FindStringIndex(config)
	if loc == nil {
		return "", errors.New("unable to find root element in folder config")
	}
	// A self closing root element has no children to insert into.
	if strings.HasSuffix(config[loc[0]:loc[1]], "/>") {
		return "", errors.New("unexpected self closing root element in folder config")
	}
	return config[:loc[1]] + element + config[loc[1]:], nil
}

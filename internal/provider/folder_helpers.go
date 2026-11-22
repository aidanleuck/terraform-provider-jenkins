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
	"io"
	"regexp"
	"strings"

	"github.com/aidanleuck/gojenkins"
)

var (
	// folderNameRegex matches a single path segment (no slashes).
	folderNameRegex = regexp.MustCompile(`^[^/]+$`)
	// folderPathRegex matches a slash separated path without leading or trailing slashes.
	folderPathRegex = regexp.MustCompile(`^[^/]+(/[^/]+)*$`)

	// xmlDeclarationRegex matches a leading XML declaration. Jenkins writes version 1.1,
	// which encoding/xml rejects, so the declaration is skipped before parsing.
	xmlDeclarationRegex = regexp.MustCompile(`^\s*<\?xml[^>]*\?>`)
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

// replaceDescription sets the description of a folder config.xml. Only the
// `description` element that is a direct child of the root element is
// replaced, so descriptions of nested elements are left untouched. If the root
// has no description it is added as the first child of the root element.
func replaceDescription(config, description string) (string, error) {
	var escaped bytes.Buffer
	if err := xml.EscapeText(&escaped, []byte(description)); err != nil {
		return "", err
	}
	element := "<description>" + escaped.String() + "</description>"

	base := 0
	if loc := xmlDeclarationRegex.FindStringIndex(config); loc != nil {
		base = loc[1]
	}
	decoder := xml.NewDecoder(strings.NewReader(config[base:]))
	depth := 0
	insertAt := -1
	descStart, descEnd := -1, -1

	for {
		before := base + int(decoder.InputOffset())
		tok, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", fmt.Errorf("failed parsing folder config: %w", err)
		}
		after := base + int(decoder.InputOffset())

		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 1 {
				// A self closing root has no children to insert into.
				if strings.HasSuffix(config[before:after], "/>") {
					return "", errors.New("unexpected self closing root element in folder config")
				}
				insertAt = after
			}
			if depth == 2 && t.Name.Local == "description" && descStart < 0 {
				descStart = before
			}
		case xml.EndElement:
			if depth == 2 && t.Name.Local == "description" && descStart >= 0 && descEnd < 0 {
				descEnd = after
			}
			depth--
		}
	}

	if descStart >= 0 && descEnd >= 0 {
		return config[:descStart] + element + config[descEnd:], nil
	}
	if insertAt < 0 {
		return "", errors.New("unable to find root element in folder config")
	}
	return config[:insertAt] + element + config[insertAt:], nil
}

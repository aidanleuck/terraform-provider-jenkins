// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitFolderPath(t *testing.T) {
	require.Nil(t, splitFolderPath(""))
	require.Equal(t, []string{"a"}, splitFolderPath("a"))
	require.Equal(t, []string{"a", "b", "c"}, splitFolderPath("a/b/c"))
}

func TestBuildFolderID(t *testing.T) {
	require.Equal(t, "child", buildFolderID("", "child"))
	require.Equal(t, "parent/child", buildFolderID("parent", "child"))
	require.Equal(t, "a/b/child", buildFolderID("a/b", "child"))
}

func TestParseFolderID(t *testing.T) {
	parent, name, err := parseFolderID("child")
	require.NoError(t, err)
	require.Equal(t, "", parent)
	require.Equal(t, "child", name)

	parent, name, err = parseFolderID("a/b/child")
	require.NoError(t, err)
	require.Equal(t, "a/b", parent)
	require.Equal(t, "child", name)

	for _, bad := range []string{"", "/a", "a/", "a//b"} {
		_, _, err = parseFolderID(bad)
		require.Error(t, err, bad)
	}
}

func TestReplaceDescription(t *testing.T) {
	const root = `<com.cloudbees.hudson.plugins.folder.Folder plugin="cloudbees-folder@1">`

	t.Run("replaces existing", func(t *testing.T) {
		in := `<?xml version='1.1' encoding='UTF-8'?>` + root + `<description>old</description><views/></com.cloudbees.hudson.plugins.folder.Folder>`
		out, err := replaceDescription(in, "new")
		require.NoError(t, err)
		require.Equal(t, `<?xml version='1.1' encoding='UTF-8'?>`+root+`<description>new</description><views/></com.cloudbees.hudson.plugins.folder.Folder>`, out)
	})

	t.Run("replaces empty element", func(t *testing.T) {
		in := root + `<description/></com.cloudbees.hudson.plugins.folder.Folder>`
		out, err := replaceDescription(in, "new")
		require.NoError(t, err)
		require.Equal(t, root+`<description>new</description></com.cloudbees.hudson.plugins.folder.Folder>`, out)
	})

	t.Run("inserts when missing", func(t *testing.T) {
		in := `<?xml version='1.1'?>` + root + `<views/></com.cloudbees.hudson.plugins.folder.Folder>`
		out, err := replaceDescription(in, "new")
		require.NoError(t, err)
		require.Equal(t, `<?xml version='1.1'?>`+root+`<description>new</description><views/></com.cloudbees.hudson.plugins.folder.Folder>`, out)
	})

	t.Run("escapes special characters", func(t *testing.T) {
		out, err := replaceDescription(root+`<description/></com.cloudbees.hudson.plugins.folder.Folder>`, `a <b> & "c"`)
		require.NoError(t, err)
		require.Contains(t, out, `<description>a &lt;b&gt; &amp; &#34;c&#34;</description>`)
	})

	t.Run("clears description", func(t *testing.T) {
		out, err := replaceDescription(root+`<description>old</description></com.cloudbees.hudson.plugins.folder.Folder>`, "")
		require.NoError(t, err)
		require.Contains(t, out, `<description></description>`)
	})

	t.Run("no root element", func(t *testing.T) {
		_, err := replaceDescription("garbage", "new")
		require.Error(t, err)
	})
}

func TestReplaceDescriptionNested(t *testing.T) {
	const root = `<com.cloudbees.hudson.plugins.folder.Folder plugin="cloudbees-folder@1">`
	const end = `</com.cloudbees.hudson.plugins.folder.Folder>`

	t.Run("nested description is not touched when root has none", func(t *testing.T) {
		in := `<?xml version='1.1' encoding='UTF-8'?>` + root + `<properties><x><description>nested</description></x></properties>` + end
		out, err := replaceDescription(in, "new")
		require.NoError(t, err)
		require.Equal(t, `<?xml version='1.1' encoding='UTF-8'?>`+root+`<description>new</description><properties><x><description>nested</description></x></properties>`+end, out)
	})

	t.Run("only the root description is replaced", func(t *testing.T) {
		in := root + `<properties><x><description>nested</description></x></properties><description>old</description>` + end
		out, err := replaceDescription(in, "new")
		require.NoError(t, err)
		require.Equal(t, root+`<properties><x><description>nested</description></x></properties><description>new</description>`+end, out)
	})

	t.Run("cdata description", func(t *testing.T) {
		in := root + `<description><![CDATA[a <b> & c]]></description><views/>` + end
		out, err := replaceDescription(in, "new")
		require.NoError(t, err)
		require.Equal(t, root+`<description>new</description><views/>`+end, out)
	})

	t.Run("description with attributes", func(t *testing.T) {
		in := root + `<description class="x">old</description>` + end
		out, err := replaceDescription(in, "new")
		require.NoError(t, err)
		require.Equal(t, root+`<description>new</description>`+end, out)
	})

	t.Run("self closing root", func(t *testing.T) {
		_, err := replaceDescription(`<a/>`, "new")
		require.Error(t, err)
	})

	t.Run("malformed", func(t *testing.T) {
		_, err := replaceDescription(root+`<description>old`, "new")
		require.Error(t, err)
	})
}

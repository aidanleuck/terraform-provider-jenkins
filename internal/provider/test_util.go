package provider

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/aidanleuck/gojenkins"
	"github.com/stretchr/testify/require"
)

type providerData struct {
	Username string
	Password string
	Server   string
}

func getProviderData(c *gojenkins.ContainerizedTest) *providerData {
	return &providerData{
		Username: c.Jenkins.Requester.BasicAuth.Username,
		Password: c.Jenkins.Requester.BasicAuth.Password,
		Server:   c.Jenkins.Server,
	}
}

func templateConfig(t *testing.T, templateStr string, data interface{}) string {
	t.Helper()
	temp := template.New("test")
	temp, err := temp.Parse(templateStr)
	require.NoError(t, err)

	var buf bytes.Buffer
	err = temp.Execute(&buf, data)
	require.NoError(t, err)

	return buf.String()
}

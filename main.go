// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/xml"
	"flag"
	"log"
	"terraform-provider-jenkins-provider/internal/provider"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

// Run "go generate" to format example terraform files and generate the docs for the registry/website

// If you do not have terraform installed, you can remove the formatting command, but its suggested to
// ensure the documentation is formatted properly.
//go:generate terraform fmt -recursive ./examples/

// Run the docs generation tool, check its repository for more information on how it works and how docs
// can be customized.
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs

var (
	// these will be set by the goreleaser configuration
	// to appropriate values for the compiled binary.
	version string = "dev"

	// goreleaser can pass other information to the main package, such as the specific commit
	// https://goreleaser.com/cookbooks/using-main.version/
)

type Jnlp struct {
	Root            xml.Name `xml:"jnlp"`
	ApplicationDesc struct {
		Argument []string `xml:"argument"`
	} `xml:"application-desc"`
}

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		// TODO: Update this string with the published name of your provider.
		Address: "registry.terraform.io/aidaleuc/jenkins-provider",
		Debug:   debug,
	}

	err := providerserver.Serve(context.Background(), provider.New(version), opts)

	if err != nil {
		log.Fatal(err.Error())
	}
}

// func main() {
// 	jenkinsServer := gojenkins.CreateJenkins(nil, "http://localhost:5000", "aidaleuc", "Fire!3355")
// 	serv, err := jenkinsServer.Init(context.Background())
// 	if err != nil {
// 		panic(err)
// 	}

// 	node, err := serv.GetNode(context.Background(), "Hello-23")
// 	if err != nil {
// 		panic(err)
// 	}
// 	jnlpAgentEndpoint := fmt.Sprintf("%s/%s/%s/%s", serv.Server, "computer", node.GetName(), "jenkins-agent.jnlp")

// 	var xmlResp Jnlp
// 	var p []byte
// 	client := http.DefaultClient
// 	req, err := http.NewRequest("Get", jnlpAgentEndpoint, nil)
// 	if err != nil {
// 		panic(err)
// 	}
// 	req.SetBasicAuth("aidaleuc", "Fire!3355")

// 	resp, err := client.Do(req)
// 	if err != nil {
// 		panic(err)
// 	}

// 	bodyText, err := ioutil.ReadAll(resp.Body)

// 	xml.Unmarshal(bodyText, &xmlResp)

// 	client.Get(jnlpAgentEndpoint)
// 	if err != nil {
// 		panic(err)
// 	}

// 	_, err = resp.Body.Read(p)
// 	if err != nil {
// 		panic(err)
// 	}

// 	println(resp.StatusCode)
// }

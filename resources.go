package ahasendmcp

import (
	"context"
	_ "embed"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ahamlReferenceURI is the URI of the AhaML reference resource.
const ahamlReferenceURI = "ahasend://docs/ahaml"

// ahamlReference is the AhaML reference that
// https://ahasend.com/docs/send-api/ahaml publishes, without that page's front
// matter. It is copied again at each release of this module and holds no text
// of its own.
//
//go:embed ahaml-reference.md
var ahamlReference string

// addResources registers the resources on a server. The AhaML reference is
// listed on every connection, so an agent without web access can read it.
func addResources(s *mcp.Server) {
	s.AddResource(&mcp.Resource{
		URI:   ahamlReferenceURI,
		Name:  "ahaml",
		Title: "AhaML reference",
		Description: "The reference of AhaSend Markup (AhaML), the XML form in which the template, campaign template " +
			"and campaign tools read and write a Simple Editor design: every tag, attribute, value, limit and error code.",
		MIMEType: "text/markdown",
		Size:     int64(len(ahamlReference)),
	}, readAhamlReference)
}

func readAhamlReference(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
		URI:      ahamlReferenceURI,
		MIMEType: "text/markdown",
		Text:     ahamlReference,
	}}}, nil
}

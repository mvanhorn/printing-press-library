package agent

import (
	"context"
	"fmt"
	"io"
)

type streamClient interface {
	GetStream(path string, params map[string]string) (io.ReadCloser, error)
}

type SalesforceFileFetcher struct {
	Client streamClient
}

func NewSalesforceFileFetcher(c streamClient) SalesforceFileFetcher {
	return SalesforceFileFetcher{Client: c}
}

func (f SalesforceFileFetcher) FetchContentVersion(ctx context.Context, contentVersionID string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f.Client == nil {
		return nil, fmt.Errorf("Salesforce client is required")
	}
	return f.Client.GetStream("/services/data/v63.0/sobjects/ContentVersion/"+contentVersionID+"/VersionData", nil)
}

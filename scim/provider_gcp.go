package scim

import (
	"context"
	"encoding/json"
	"fmt"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

type GcpProvider struct {
	SecretName string
}

func NewGcpProvider(secretName string) SecretProvider {
	return &GcpProvider{
		SecretName: secretName,
	}
}

func (p *GcpProvider) Load() (*ScimEndpointParameters, *GoogleEndpointParameters, error) {
	ctx := context.Background()
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create GCP secretmanager client: %w", err)
	}
	defer client.Close()

	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: p.SecretName,
	}

	result, err := client.AccessSecretVersion(ctx, req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to access GCP secret version (ensure service account has Secret Manager Secret Accessor role): %w", err)
	}

	payload := result.Payload.Data

	var scimParams ScimEndpointParameters
	if err := json.Unmarshal(payload, &scimParams); err != nil {
		return nil, nil, fmt.Errorf("failed to parse SCIM parameters from GCP secret: %w", err)
	}

	var googleParams GoogleEndpointParameters
	if err := json.Unmarshal(payload, &googleParams); err != nil {
		return nil, nil, fmt.Errorf("failed to parse Google parameters from GCP secret: %w", err)
	}

	return &scimParams, &googleParams, nil
}

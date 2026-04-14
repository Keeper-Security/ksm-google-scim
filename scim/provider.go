package scim

type SecretProvider interface {
	Load() (*ScimEndpointParameters, *GoogleEndpointParameters, error)
}

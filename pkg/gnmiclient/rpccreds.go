package gnmiclient

import (
	"context"
	"google.golang.org/grpc/credentials"
)

// perRpcCreds represents per RPC credentials.
type perRpcCreds struct {
	username string
	password string
	secure   bool
}

// GetRequestMetadata implements the required credentials interface
func (c *perRpcCreds) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"username": c.username,
		"password": c.password,
	}, nil
}

// RequireTransportSecurity implements the required credentials interface
func (c *perRpcCreds) RequireTransportSecurity() bool {
	return c.secure
}

// newPerRpcCreds creates a new instance of perRpcCreds, used for dialing the target device.
func newPerRpcCreds(user, pwd string, secure bool) credentials.PerRPCCredentials {
	return &perRpcCreds{
		username: user,
		password: pwd,
		secure:   secure,
	}
}

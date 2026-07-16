package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	vault "github.com/hashicorp/vault/api"
)

// vaultClient wraps the Vault/OpenBao API client with the operations this
// provider needs to manage the gpg secrets engine and its keys.
type vaultClient struct {
	api *vault.Client
}

func newVaultClient(address, token string) (*vaultClient, error) {
	cfg := vault.DefaultConfig()
	if cfg.Error != nil {
		return nil, cfg.Error
	}
	if address != "" {
		cfg.Address = address
	}
	c, err := vault.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	if token != "" {
		c.SetToken(token)
	}
	return &vaultClient{api: c}, nil
}

// --- plugin catalog ---

// registerPlugin registers (or updates) a secret plugin in the catalog.
func (c *vaultClient) registerPlugin(ctx context.Context, name, command, sha256 string) error {
	return c.api.Sys().RegisterPluginWithContext(ctx, &vault.RegisterPluginInput{
		Name:    name,
		Type:    vault.PluginTypeSecrets,
		Command: command,
		SHA256:  sha256,
	})
}

// deregisterPlugin removes a secret plugin from the catalog.
func (c *vaultClient) deregisterPlugin(ctx context.Context, name string) error {
	return c.api.Sys().DeregisterPluginWithContext(ctx, &vault.DeregisterPluginInput{
		Name: name,
		Type: vault.PluginTypeSecrets,
	})
}

// pluginInfo returns the catalog entry for a secret plugin, or nil if absent.
func (c *vaultClient) pluginInfo(ctx context.Context, name string) (*vault.GetPluginResponse, error) {
	info, err := c.api.Sys().GetPluginWithContext(ctx, &vault.GetPluginInput{
		Name: name,
		Type: vault.PluginTypeSecrets,
	})
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return info, nil
}

// --- mounts ---

func (c *vaultClient) enableMount(ctx context.Context, path, pluginType, description string) error {
	return c.api.Sys().MountWithContext(ctx, path, &vault.MountInput{
		Type:        pluginType,
		Description: description,
	})
}

func (c *vaultClient) tuneMount(ctx context.Context, path, description string) error {
	return c.api.Sys().TuneMountWithContext(ctx, path, vault.MountConfigInput{
		Description: &description,
	})
}

// mountInfo returns the mount at path, or nil if it does not exist.
func (c *vaultClient) mountInfo(ctx context.Context, path string) (*vault.MountOutput, error) {
	mounts, err := c.api.Sys().ListMountsWithContext(ctx)
	if err != nil {
		return nil, err
	}
	key := strings.TrimRight(path, "/") + "/"
	if m, ok := mounts[key]; ok {
		return m, nil
	}
	return nil, nil
}

func (c *vaultClient) disableMount(ctx context.Context, path string) error {
	return c.api.Sys().UnmountWithContext(ctx, path)
}

// --- keys ---

func (c *vaultClient) writeKey(ctx context.Context, backend, name string, data map[string]interface{}) (map[string]interface{}, error) {
	secret, err := c.api.Logical().WriteWithContext(ctx, keyPath(backend, name), data)
	if err != nil {
		return nil, err
	}
	if secret == nil {
		return nil, nil
	}
	return secret.Data, nil
}

// readKey reads a key's metadata, returning nil if it does not exist.
func (c *vaultClient) readKey(ctx context.Context, backend, name string) (map[string]interface{}, error) {
	secret, err := c.api.Logical().ReadWithContext(ctx, keyPath(backend, name))
	if err != nil {
		return nil, err
	}
	if secret == nil {
		return nil, nil
	}
	return secret.Data, nil
}

func (c *vaultClient) writeKeyConfig(ctx context.Context, backend, name string, data map[string]interface{}) error {
	_, err := c.api.Logical().WriteWithContext(ctx, keyPath(backend, name)+"/config", data)
	return err
}

func (c *vaultClient) deleteKey(ctx context.Context, backend, name string) error {
	_, err := c.api.Logical().DeleteWithContext(ctx, keyPath(backend, name))
	return err
}

func keyPath(backend, name string) string {
	return fmt.Sprintf("%s/keys/%s", strings.TrimRight(backend, "/"), name)
}

// isMountAlreadyExists reports whether the error is Vault's "path is already in
// use" response.
func isMountAlreadyExists(err error) bool {
	return responseContains(err, "path is already in use")
}

// isNotFound reports a 404-style response from Vault.
func isNotFound(err error) bool {
	var respErr *vault.ResponseError
	if errors.As(err, &respErr) {
		return respErr.StatusCode == 404
	}
	return false
}

func responseContains(err error, substr string) bool {
	if err == nil {
		return false
	}
	var respErr *vault.ResponseError
	if errors.As(err, &respErr) {
		for _, e := range respErr.Errors {
			if strings.Contains(e, substr) {
				return true
			}
		}
	}
	return false
}

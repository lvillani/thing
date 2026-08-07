// SPDX-License-Identifier: GPL-3.0-only

// Package keychain provides a simple interface for storing and retrieving secrets from
// the system keychain.
package keychain

import (
	"errors"

	"github.com/zalando/go-keyring"
)

const (
	service = "thing"
	account = "api"
)

// GetApiToken retrieves the API token from the system keychain. It returns an error if
// the token is not found or if there is an issue accessing the keychain.
func GetApiToken() (string, error) {
	token, err := keyring.Get(service, account)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return "", errors.New("API token not found in keychain")
		}
		return "", err
	}

	return token, nil
}

// StoreApiToken stores the given API token in the system keychain.
func StoreApiToken(token string) error {
	return keyring.Set(service, account, token)
}

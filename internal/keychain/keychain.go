// SPDX-License-Identifier: GPL-3.0-only

// Package keychain provides a simple interface for storing and retrieving secrets from
// the system keychain.
package keychain

import (
	"errors"

	"github.com/keybase/go-keychain"
)

// GetApiToken retrieves the API token from the system keychain. It returns an error if
// the token is not found or if there is an issue accessing the keychain.
func GetApiToken() (string, error) {
	token, err := keychain.GetGenericPassword("thing", "api", "", "")
	if token == nil {
		return "", errors.New("API token not found in keychain")
	}
	if err != nil {
		return "", err
	}

	return string(token), nil
}

// StoreApiToken stores the given API token in the system keychain.
func StoreApiToken(token string) error {
	item := keychain.NewGenericPassword("thing", "api", "", []byte(token), "")
	if err := keychain.AddItem(item); err != nil {
		return err
	}

	return nil
}

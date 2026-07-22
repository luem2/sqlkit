package config

import "github.com/zalando/go-keyring"

const keyringService = "sqlkit"

var (
	keyringGet = keyring.Get
	keyringSet = keyring.Set
)

func KeyringKey(envName string) string {
	normalized, err := NormalizeEnvName(envName)
	if err != nil {
		normalized = envName
	}
	return "env/" + normalized + "/password"
}

func SecretKey(name string) string {
	return "secret/" + name
}

func SetSecret(key string, value string) error {
	return keyringSet(keyringService, key, value)
}

func Secret(key string) (string, error) {
	return keyringGet(keyringService, key)
}

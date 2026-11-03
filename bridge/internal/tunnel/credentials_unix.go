//go:build !windows

package tunnel

func encodeCredentials(key string) (persistedCredentials, error) {
	return persistedCredentials{APIKey: key}, nil
}
func decodeCredentials(saved persistedCredentials) (string, error) { return saved.APIKey, nil }

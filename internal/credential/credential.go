package credential

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Store struct {
	Dir string
}

func (s Store) Save(key, secret string) (string, error) {
	key = sanitizeKey(key)
	if key == "" {
		return "", errors.New("empty credential key")
	}
	protected, err := protect([]byte(secret))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(s.Dir, key+".bin")
	if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(protected)), 0o600); err != nil {
		return "", err
	}
	return "dpapi:" + key, nil
}

func (s Store) Load(ref string) (string, error) {
	key := strings.TrimPrefix(ref, "dpapi:")
	key = sanitizeKey(key)
	if key == "" {
		return "", errors.New("empty credential reference")
	}
	data, err := os.ReadFile(filepath.Join(s.Dir, key+".bin"))
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		return "", err
	}
	plain, err := unprotect(raw)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (s Store) Delete(ref string) error {
	key := strings.TrimPrefix(ref, "dpapi:")
	key = sanitizeKey(key)
	if key == "" {
		return nil
	}
	err := os.Remove(filepath.Join(s.Dir, key+".bin"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func sanitizeKey(key string) string {
	key = strings.TrimSpace(strings.ReplaceAll(key, "\\", "_"))
	key = strings.ReplaceAll(key, "/", "_")
	key = strings.ReplaceAll(key, ":", "_")
	return key
}

func platformName() string {
	return runtime.GOOS
}

func unsupportedDPAPI() error {
	return fmt.Errorf("DPAPI credential storage is only supported on Windows, current platform: %s", platformName())
}

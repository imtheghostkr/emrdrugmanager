//go:build !windows

package credential

func protect(data []byte) ([]byte, error) {
	return nil, unsupportedDPAPI()
}

func unprotect(data []byte) ([]byte, error) {
	return nil, unsupportedDPAPI()
}

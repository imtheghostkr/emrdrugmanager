//go:build windows

package credential

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

var (
	crypt32              = windows.NewLazySystemDLL("crypt32.dll")
	kernel32             = windows.NewLazySystemDLL("kernel32.dll")
	procCryptProtectData = crypt32.NewProc("CryptProtectData")
	procCryptUnprotect   = crypt32.NewProc("CryptUnprotectData")
	procLocalFree        = kernel32.NewProc("LocalFree")
)

func protect(data []byte) ([]byte, error) {
	in := bytesToBlob(data)
	var out dataBlob
	r, _, err := procCryptProtectData.Call(
		uintptr(unsafe.Pointer(&in)),
		0,
		0,
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return nil, err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return blobToBytes(out), nil
}

func unprotect(data []byte) ([]byte, error) {
	in := bytesToBlob(data)
	var out dataBlob
	r, _, err := procCryptUnprotect.Call(
		uintptr(unsafe.Pointer(&in)),
		0,
		0,
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return nil, err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return blobToBytes(out), nil
}

func bytesToBlob(data []byte) dataBlob {
	if len(data) == 0 {
		return dataBlob{}
	}
	return dataBlob{cbData: uint32(len(data)), pbData: &data[0]}
}

func blobToBytes(blob dataBlob) []byte {
	if blob.cbData == 0 || blob.pbData == nil {
		return nil
	}
	src := unsafe.Slice(blob.pbData, blob.cbData)
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}

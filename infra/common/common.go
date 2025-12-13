package common

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func GenerateHash(path string) (string, error) {
	var hash string

	err := filepath.Walk(path,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() && info.Mode()&os.ModeSymlink != os.ModeSymlink {
				fh, err := GetFileMd5Hash(path)
				if err != nil {
					return err
				}
				hash = AppendHash(hash, fh)
			}

			return nil
		})

	return hash, err
}

func GetFileMd5Hash(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}

	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func AppendHash(hash1, hash2 string) string {
	h := md5.New()
	io.WriteString(h, fmt.Sprintf("%s%s", hash1, hash2))

	return fmt.Sprintf("%x", h.Sum(nil))
}

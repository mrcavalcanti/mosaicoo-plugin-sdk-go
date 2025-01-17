package ca

import (
	_ "embed" // Needed for embedding the CA certificate and key

	"crypto/tls"
	"io/ioutil"
)

//go:embed mosaicoo-e2e-ca.pem
// CACertificate Certificate Authority certificate used by the proxy.
var CACertificate []byte

//go:embed mosaicoo-e2e-ca.key.pem
// CAKey Certificate Authority private key used by the proxy.
var CAKey []byte

// Loads the CA key pair from the provided paths, and falls back to the default key pair if paths are not provided.
func GetCertificate(certPath, keyPath string) (tls.Certificate, error) {
	if certPath == "" || keyPath == "" {
		return tls.X509KeyPair(CACertificate, CAKey)
	}
	cert, key, err := LoadKeyPair(certPath, keyPath)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.X509KeyPair(cert, key)
}

// Loads the CA key pair from the provided paths.
func LoadKeyPair(certPath, keyPath string) ([]byte, []byte, error) {
	cert, err := ioutil.ReadFile(certPath)
	if err != nil {
		return []byte{}, []byte{}, err
	}

	key, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return []byte{}, []byte{}, err
	}

	return cert, key, nil
}

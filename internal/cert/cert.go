// Package cert handles certificates for https proxied requests
package cert

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"
)

const (
	caMaxAge   = 3 * 365 * 24 * time.Hour
	leafMaxAge = 7 * 24 * time.Hour
	caKeyUsage = x509.KeyUsageDigitalSignature |
		x509.KeyUsageContentCommitment |
		x509.KeyUsageKeyEncipherment |
		x509.KeyUsageDataEncipherment |
		x509.KeyUsageKeyAgreement |
		x509.KeyUsageCertSign |
		x509.KeyUsageCRLSign
	keyUsage = x509.KeyUsageDigitalSignature |
		x509.KeyUsageContentCommitment |
		x509.KeyUsageKeyEncipherment |
		x509.KeyUsageDataEncipherment
)

type Cert struct {
	ca     *tls.Certificate
	leafs  map[string]*tls.Certificate
	serial *big.Int
}

func NewCert() *Cert {
	c := Cert{
		leafs:  map[string]*tls.Certificate{},
		serial: big.NewInt(time.Now().Unix() << 16), // leave room for lots of certificates
	}
	return &c
}

func (c *Cert) CAGen(name string) error {
	var key, pubKey crypto.PrivateKey
	if c.ca != nil {
		key = c.ca.PrivateKey
		pubKey = c.ca.Leaf.PublicKey
	} else {
		// k, err := genKeyRSA(4096)
		k, err := genKeyEC()
		if err != nil {
			return err
		}
		key = k
		pubKey = k.Public()
	}
	now := time.Now().UTC()
	tmpl := x509.Certificate{
		SerialNumber:          c.serial,
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             now.Add(-1 * time.Hour),
		NotAfter:              now.Add(caMaxAge),
		KeyUsage:              caKeyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
		MaxPathLenZero:        false,
		// SignatureAlgorithm:    x509.ECDSAWithSHA1,
		SignatureAlgorithm: x509.ECDSAWithSHA512,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, pubKey, key)
	if err != nil {
		return err
	}
	certX509, err := x509.ParseCertificate(certDER)
	if err != nil {
		return err
	}

	ca := tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
		Leaf:        certX509,
	}
	c.serial.Add(c.serial, big.NewInt(1))
	return c.CASet(&ca)
}

func (c *Cert) CARead(keyRdr, certRdr io.Reader) error {
	keyBytes, err := io.ReadAll(keyRdr)
	if err != nil {
		return err
	}
	certBytes, err := io.ReadAll(certRdr)
	if err != nil {
		return err
	}
	cert, err := tls.X509KeyPair(keyBytes, certBytes)
	if err != nil {
		return err
	}
	return c.CASet(&cert)
}

func (c *Cert) CASet(cert *tls.Certificate) error {
	if c.ca != nil {
		return fmt.Errorf("CA already configured")
	}
	if len(cert.Certificate) < 1 || len(cert.Certificate[0]) == 0 {
		return fmt.Errorf("CA certificate must be included")
	}
	if cert.PrivateKey == nil {
		return fmt.Errorf("CA private key must be included")
	}
	if cert.Leaf == nil {
		return fmt.Errorf("CA cert missing leaf for public key")
	}
	if !cert.Leaf.IsCA {
		return fmt.Errorf("CA cert is missing CA flag")
	}
	c.ca = cert
	return nil
}

func (c *Cert) CAGetPEM() ([]byte, error) {
	if c.ca == nil {
		return nil, fmt.Errorf("CA not configured")
	}
	if len(c.ca.Certificate) < 1 {
		return nil, fmt.Errorf("CA certificate not available")
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: c.ca.Certificate[0],
	}), nil
}

func (c *Cert) LeafCert(names []string) (*tls.Certificate, error) {
	now := time.Now().UTC()
	if len(names) < 1 {
		return nil, fmt.Errorf("missing name for leaf cert")
	}
	if c.ca == nil || c.ca.Leaf == nil || !c.ca.Leaf.IsCA {
		return nil, fmt.Errorf("CA needed to generate leaf certificates")
	}
	namesJoin := strings.Join(names, ", ")
	if cert, ok := c.leafs[namesJoin]; ok && now.Add(time.Hour).Before(cert.Leaf.NotAfter) {
		return cert, nil
	}

	key, err := genKeyEC()
	// key, err := genKeyRSA(2048)
	if err != nil {
		return nil, err
	}
	tmpl := x509.Certificate{
		SerialNumber:          c.serial,
		Subject:               pkix.Name{CommonName: names[0]},
		NotBefore:             now.Add(-1 * time.Hour),
		NotAfter:              now.Add(leafMaxAge),
		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		DNSNames:              names,
		// SignatureAlgorithm:    x509.ECDSAWithSHA1,
		SignatureAlgorithm: x509.ECDSAWithSHA512,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &tmpl, c.ca.Leaf, key.Public(), c.ca.PrivateKey)
	if err != nil {
		return nil, err
	}
	certX509, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, err
	}
	leaf := tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
		Leaf:        certX509,
	}
	c.serial.Add(c.serial, big.NewInt(1))
	c.leafs[namesJoin] = &leaf
	return &leaf, nil
}

func genKeyEC() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
}

//lint:ignore U1000 saving for potential future usage
func genKeyRSA(bits int) (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, bits)
}

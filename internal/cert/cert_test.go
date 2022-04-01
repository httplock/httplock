package cert

import (
	"crypto/x509"
	"encoding/pem"
	"testing"
)

func TestCert(t *testing.T) {
	c := NewCert()
	exampleHost := "example.org"

	exCert, err := c.LeafCert([]string{exampleHost})
	if err == nil {
		t.Errorf("Unexpected success generating leaf cert, %v", exCert)
	}

	err = c.CAGen("Reproducible Test")
	if err != nil {
		t.Errorf("Failed to generate CA cert: %v", err)
	}

	buf, err := c.CAGetPEM()
	if err != nil {
		t.Errorf("Failed to write CA: %v", err)
	}
	t.Logf("Generated CA:\n%s", buf)

	pool := x509.NewCertPool()
	ok := pool.AppendCertsFromPEM(buf)
	if !ok {
		t.Errorf("Failed to add CA to cert pool")
	}

	exCert, err = c.LeafCert([]string{exampleHost})
	if err != nil {
		t.Errorf("Failed to generate leaf cert, %v", err)
	}
	t.Logf("Leaf cert:\n%s", pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: exCert.Certificate[0],
	}))

	exCert2, err := c.LeafCert([]string{exampleHost})
	if err != nil {
		t.Errorf("Failed to get leaf cert from cache, %v", err)
	}

	if !exCert.Leaf.Equal(exCert2.Leaf) {
		t.Errorf("Leaf certificates for %s do not match: %v should equal %v", exampleHost, exCert.Leaf, exCert2.Leaf)
	}

	if exCert.Leaf.IsCA {
		t.Errorf("Leaf cert returned with IsCA set")
	}
	err = exCert.Leaf.VerifyHostname(exampleHost)
	if err != nil {
		t.Errorf("Leaf cert is not valid for %s, err: %v, dnsNames: %v", exampleHost, err, exCert.Leaf.DNSNames)
	}

	// verify ca certificate to itself
	block, _ := pem.Decode(buf)
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Errorf("Failed to parse ca cert: %v", err)
	}
	//lint:ignore SA4006 testing still being developed
	chains, err := caCert.Verify(x509.VerifyOptions{
		Roots: pool,
		// DNSName:   exampleHost,
		// KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})
	if err != nil {
		t.Errorf("Failed to verify ca cert: %v", err)
	}

	// TODO: track down why verification fails
	intermediates := x509.NewCertPool()
	intermediates.AppendCertsFromPEM(buf)
	chains, err = exCert.Leaf.Verify(x509.VerifyOptions{
		Roots:         pool,
		Intermediates: pool,
		// DNSName:   exampleHost,
		// KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})
	if err != nil {
		t.Errorf("Failed to verify leaf cert: %v", err)
	}
	_ = chains
	// t.Logf("Verification chains: %v", chains)

	// TODO: sign some data with leaf, verify with ca cert
	// msg := "Hello world"
	// signer, ok := exCert.PrivateKey.(crypto.Signer)
	// if !ok {
	// 	t.Errorf("Private key is not a crypto.Signer")
	// 	return
	// }
	// sig, err := signer.Sign(rand.Reader, []byte(msg), nil)
	// if err != nil {
	// 	t.Errorf("signer.Sign: %v", err)
	// }

	// decrypter, ok := exCert.Leaf.PublicKey.(crypto.Decrypter)
	// if !ok {
	// 	t.Errorf("Public key is not a crypto.Decrypter")
	// 	return
	// }
	// plaintext, err := decrypter.Decrypt(rand.Reader, sig, nil)
	// if msg != string(plaintext) {
	// 	t.Errorf("Decrypted message does not match: expect %s, received %s", msg, plaintext)
	// }

}

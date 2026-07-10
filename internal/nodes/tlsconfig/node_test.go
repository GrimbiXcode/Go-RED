package tlsconfig

import (
    "crypto/ecdsa"
    "crypto/elliptic"
    "crypto/rand"
    "crypto/x509"
    "crypto/x509/pkix"
    "encoding/pem"
    "math/big"
    "os"
    "path/filepath"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// generateSelfSignedPEM returns a fresh self-signed cert/key pair as PEM
// bytes, for exercising TLSConfig without depending on external tooling.
func generateSelfSignedPEM(t *testing.T) (certPEM, keyPEM []byte) {
    t.Helper()
    priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
    require.NoError(t, err)

    template := x509.Certificate{
        SerialNumber: big.NewInt(1),
        Subject:      pkix.Name{CommonName: "go-red-test"},
        NotBefore:    time.Now().Add(-time.Hour),
        NotAfter:     time.Now().Add(time.Hour),
        KeyUsage:     x509.KeyUsageDigitalSignature,
        ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
    }
    der, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
    require.NoError(t, err)

    certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
    keyDER, err := x509.MarshalECPrivateKey(priv)
    require.NoError(t, err)
    keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
    return certPEM, keyPEM
}

func TestNode_ConfigRoundTrip(t *testing.T) {
    n := &Node{}
    require.NoError(t, n.SetConfig(map[string]interface{}{
        "certType":         "pem",
        "cert":             "cert-data",
        "key":              "key-data",
        "ca":               "ca-data",
        "servername":       "example.com",
        "verifyServerCert": false,
    }))
    assert.Equal(t, "pem", n.CertType)
    assert.Equal(t, "cert-data", n.Cert)
    assert.Equal(t, "key-data", n.Key)
    assert.Equal(t, "ca-data", n.CA)
    assert.Equal(t, "example.com", n.ServerName)
    assert.False(t, n.VerifyServerCert)
}

func TestNode_Validate(t *testing.T) {
    assert.Error(t, (&Node{CertType: "bogus"}).Validate())
    assert.NoError(t, (&Node{CertType: "files"}).Validate())
    assert.NoError(t, (&Node{}).Validate())
}

func TestNode_TLSConfig_NoMaterial(t *testing.T) {
    n := &Node{ServerName: "example.com", VerifyServerCert: true}
    cfg, err := n.TLSConfig()
    require.NoError(t, err)
    assert.Equal(t, "example.com", cfg.ServerName)
    assert.False(t, cfg.InsecureSkipVerify)
    assert.Empty(t, cfg.Certificates)
}

func TestNode_TLSConfig_InsecureSkipVerify(t *testing.T) {
    n := &Node{VerifyServerCert: false}
    cfg, err := n.TLSConfig()
    require.NoError(t, err)
    assert.True(t, cfg.InsecureSkipVerify)
}

func TestNode_TLSConfig_PEMMode(t *testing.T) {
    certPEM, keyPEM := generateSelfSignedPEM(t)
    n := &Node{CertType: "pem", Cert: string(certPEM), Key: string(keyPEM), CA: string(certPEM)}
    cfg, err := n.TLSConfig()
    require.NoError(t, err)
    require.Len(t, cfg.Certificates, 1)
    require.NotNil(t, cfg.RootCAs)
}

func TestNode_TLSConfig_FilesMode(t *testing.T) {
    certPEM, keyPEM := generateSelfSignedPEM(t)
    dir := t.TempDir()
    certPath := filepath.Join(dir, "cert.pem")
    keyPath := filepath.Join(dir, "key.pem")
    require.NoError(t, os.WriteFile(certPath, certPEM, 0o644))
    require.NoError(t, os.WriteFile(keyPath, keyPEM, 0o644))

    n := &Node{CertType: "files", Cert: certPath, Key: keyPath}
    cfg, err := n.TLSConfig()
    require.NoError(t, err)
    require.Len(t, cfg.Certificates, 1)
}

func TestNode_TLSConfig_FilesMode_MissingFile(t *testing.T) {
    n := &Node{CertType: "files", Cert: "/nonexistent/cert.pem", Key: "/nonexistent/key.pem"}
    _, err := n.TLSConfig()
    assert.Error(t, err)
}

func TestNode_TLSConfig_EnvMode(t *testing.T) {
    certPEM, keyPEM := generateSelfSignedPEM(t)
    t.Setenv("GORED_TEST_CERT", string(certPEM))
    t.Setenv("GORED_TEST_KEY", string(keyPEM))

    n := &Node{CertType: "env", Cert: "GORED_TEST_CERT", Key: "GORED_TEST_KEY"}
    cfg, err := n.TLSConfig()
    require.NoError(t, err)
    require.Len(t, cfg.Certificates, 1)
}

func TestNode_Execute_NotSupported(t *testing.T) {
    _, err := (&Node{}).Execute(nil, map[string]interface{}{})
    assert.Error(t, err)
}

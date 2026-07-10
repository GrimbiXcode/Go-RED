// Package tlsconfig provides the "tls-config" config node - a reusable TLS
// client/server certificate configuration referenced by ID from other
// nodes (http request, mqtt-broker, websocket-client), rather than being
// wired into a flow itself (see docs/NODE_PALETTE_PLAN.md, Phase 6's
// config-node concept).
//
// A consuming node resolves this node's live instance via
// registry.NodeRuntime.GetNode(configNodeID) (populated from a plain
// string config property, e.g. Config["tls"] = "<tls-config node ID>") and
// type-asserts it to Provider - a direct Go import between the two node
// packages, not a registry-mediated interface, since only a handful of
// node types need it.
//
// Node-RED's PKCS#12 (.pfx/.p12) certificate bundles are not supported
// here (would need an extra dependency, golang.org/x/crypto/pkcs12, for a
// format most deployments don't use) - see docs/NODE_PALETTE_PLAN.md for
// the documented scope cut. CertType "files"/"pem"/"env" cover the same
// three ways Node-RED lets a cert/key/CA reach this node (a path on disk,
// inline PEM text, or an env var holding PEM text).
package tlsconfig

import (
    "crypto/tls"
    "crypto/x509"
    "fmt"
    "os"

    "github.com/GrimbiXcode/Go-RED/internal/registry"
)

// Provider is implemented by tls-config nodes (this package's Node),
// exposing a ready-to-use *tls.Config to a consuming node.
type Provider interface {
    TLSConfig() (*tls.Config, error)
}

// Node holds a TLS-config node's configuration.
type Node struct {
    // CertType is "files" (Cert/Key/CA are filesystem paths), "pem"
    // (Cert/Key/CA are literal PEM text), or "env" (Cert/Key/CA name
    // environment variables holding PEM text). Default "files".
    CertType string
    Cert     string
    Key      string
    CA       string
    // ServerName overrides the TLS ServerName (SNI) sent to the peer;
    // empty uses the connection's own target host.
    ServerName string
    // VerifyServerCert, if false, sets InsecureSkipVerify - only for
    // testing against a self-signed peer, never recommended in production.
    VerifyServerCert bool
}

// TLSConfig builds a *tls.Config from Node's configuration, loading
// Cert/Key/CA per CertType. Returns a zero-value-safe *tls.Config with just
// ServerName/InsecureSkipVerify set if no certificate is configured (valid
// for a client that only needs to verify the server, not present a client
// certificate).
func (n *Node) TLSConfig() (*tls.Config, error) {
    cfg := &tls.Config{
        ServerName:         n.ServerName,
        InsecureSkipVerify: !n.VerifyServerCert,
    }

    certPEM, keyPEM, caPEM, err := n.loadMaterial()
    if err != nil {
        return nil, err
    }

    if len(certPEM) > 0 || len(keyPEM) > 0 {
        cert, err := tls.X509KeyPair(certPEM, keyPEM)
        if err != nil {
            return nil, fmt.Errorf("tls-config: %w", err)
        }
        cfg.Certificates = []tls.Certificate{cert}
    }
    if len(caPEM) > 0 {
        pool := x509.NewCertPool()
        if !pool.AppendCertsFromPEM(caPEM) {
            return nil, fmt.Errorf("tls-config: no valid certificates found in CA PEM data")
        }
        cfg.RootCAs = pool
    }

    return cfg, nil
}

func (n *Node) loadMaterial() (certPEM, keyPEM, caPEM []byte, err error) {
    switch n.CertType {
    case "", "files":
        if certPEM, err = readIfSet(n.Cert); err != nil {
            return nil, nil, nil, err
        }
        if keyPEM, err = readIfSet(n.Key); err != nil {
            return nil, nil, nil, err
        }
        if caPEM, err = readIfSet(n.CA); err != nil {
            return nil, nil, nil, err
        }
        return certPEM, keyPEM, caPEM, nil
    case "pem":
        return []byte(n.Cert), []byte(n.Key), []byte(n.CA), nil
    case "env":
        return []byte(os.Getenv(n.Cert)), []byte(os.Getenv(n.Key)), []byte(os.Getenv(n.CA)), nil
    default:
        return nil, nil, nil, fmt.Errorf("tls-config: unsupported certType %q", n.CertType)
    }
}

func readIfSet(path string) ([]byte, error) {
    if path == "" {
        return nil, nil
    }
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("tls-config: read %s: %w", path, err)
    }
    return data, nil
}

// Execute exists only to satisfy registry.NodeExecutor - a config node has
// no message inputs/outputs of its own and is never wired into a flow's
// message path.
func (n *Node) Execute(ctx interface{}, input map[string]interface{}) (map[string]interface{}, error) {
    return nil, fmt.Errorf("tls-config: config nodes have no message input")
}

func (n *Node) Validate() error {
    switch n.CertType {
    case "", "files", "pem", "env":
        return nil
    default:
        return fmt.Errorf("tls-config: unsupported certType %q", n.CertType)
    }
}

func (n *Node) GetConfig() map[string]interface{} {
    return map[string]interface{}{
        "certType":         n.CertType,
        "cert":             n.Cert,
        "key":              n.Key,
        "ca":               n.CA,
        "servername":       n.ServerName,
        "verifyServerCert": n.VerifyServerCert,
    }
}

func (n *Node) SetConfig(config map[string]interface{}) error {
    n.CertType = "files"
    if ct, ok := config["certType"].(string); ok && ct != "" {
        n.CertType = ct
    }
    if v, ok := config["cert"].(string); ok {
        n.Cert = v
    }
    if v, ok := config["key"].(string); ok {
        n.Key = v
    }
    if v, ok := config["ca"].(string); ok {
        n.CA = v
    }
    if v, ok := config["servername"].(string); ok {
        n.ServerName = v
    }
    if v, ok := config["verifyServerCert"].(bool); ok {
        n.VerifyServerCert = v
    }
    return n.Validate()
}

func init() {
    reg := registry.GetGlobalRegistry()
    err := reg.RegisterFactory("tls-config", func() registry.NodeExecutor {
        return &Node{CertType: "files", VerifyServerCert: true}
    }, registry.NodeMetadata{
        ID:          "tls-config",
        Type:        "tls-config",
        Name:        "TLS Config",
        Description: "Reusable TLS certificate/key/CA configuration, referenced by ID from http request/mqtt-broker/websocket-client",
        Category:    "config",
        Inputs:      []registry.Port{},
        Outputs:     []registry.Port{},
        ConfigSchema: registry.Schema{
            Properties: map[string]registry.Property{
                "certType":         {Type: "string", Description: "files (paths on disk), pem (literal PEM text), or env (env var names holding PEM text)", Default: "files", Enum: []string{"files", "pem", "env"}},
                "cert":             {Type: "string", Description: "Client certificate: path, PEM text, or env var name depending on certType", Default: ""},
                "key":              {Type: "string", Description: "Client private key: path, PEM text, or env var name depending on certType", Default: ""},
                "ca":               {Type: "string", Description: "CA certificate: path, PEM text, or env var name depending on certType", Default: ""},
                "servername":       {Type: "string", Description: "Override the TLS ServerName (SNI) sent to the peer", Default: ""},
                "verifyServerCert": {Type: "boolean", Description: "Verify the peer's certificate (disable only for testing against a self-signed peer)", Default: true},
            },
        },
        Icon: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#00ADD8"><path d="M12 2l8 4v6c0 5-3.5 8.5-8 10-4.5-1.5-8-5-8-10V6z"/></svg>`,
        Tags: []string{"config", "tls", "ssl", "certificate"},
    })
    if err != nil {
        panic(err)
    }
}

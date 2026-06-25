package pp

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewWechatPaymentProviderWithRawWechatPayPublicKey(t *testing.T) {
	privateKey, publicKey := newWechatPayTestKeys(t)
	publicKeyId := "PUB_KEY_ID_TEST"

	provider, err := NewWechatPaymentProvider(
		"1900000001",
		strings.Repeat("a", 32),
		"wx1234567890abcdef",
		"merchant-serial-no",
		privateKey,
		publicKeyId,
		stripWechatPayTestPEM(publicKey),
	)

	require.NoError(t, err)
	require.NotNil(t, provider.Client)
	require.Equal(t, publicKeyId, provider.Client.WxSerialNo)
}

func newWechatPayTestKeys(t *testing.T) (string, string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	privateKey := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	publicKey := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	return string(privateKey), string(publicKey)
}

func stripWechatPayTestPEM(value string) string {
	lines := strings.Split(value, "\n")
	res := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "-----") {
			continue
		}
		res = append(res, line)
	}
	return strings.Join(res, "")
}

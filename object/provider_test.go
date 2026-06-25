package object

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetWechatPayPublicKeyConfigFromMetadata(t *testing.T) {
	publicKeyId, publicKey := getWechatPayPublicKeyConfigFromMetadata(`{
		"wechatPayPublicKeyId": "PUB_KEY_ID_TEST",
		"wechatPayPublicKey": "PUBLIC_KEY_BODY"
	}`)

	require.Equal(t, "PUB_KEY_ID_TEST", publicKeyId)
	require.Equal(t, "PUBLIC_KEY_BODY", publicKey)
}

func TestGetWechatPayPublicKeyConfigFromMetadataIgnoresInvalidJson(t *testing.T) {
	publicKeyId, publicKey := getWechatPayPublicKeyConfigFromMetadata(`not-json`)

	require.Empty(t, publicKeyId)
	require.Empty(t, publicKey)
}

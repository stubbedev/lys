package service

import (
	"fmt"
	"io"

	"github.com/stubbedev/lys/secrets/transport"
)

// maxDHTries bounds the private keys negotiateDH draws; each is
// rejected with probability 1/256.
const maxDHTries = 64

// negotiateDH answers a client's public key: the server's public key to
// send back, and the session's transport. The shared secret is stretched
// with HKDF-SHA256 (no salt, no info) to an AES-128 key.
//
// Clients disagree on the secret's encoding: libsecret built on libgcrypt,
// gnome-keyring and python-secretstorage pad it to the prime's length,
// libsecret built on GnuTLS drops its leading zero bytes. The private key
// is drawn until the secret has no leading zero byte, where the two
// encodings are the same bytes, so every client derives this key.
func negotiateDH(peer []byte, random io.Reader) ([]byte, transport.Transport, error) {
	for range maxDHTries {
		priv, public, err := transport.KeyPair(random)
		if err != nil {
			return nil, nil, err
		}
		shared, err := transport.Shared(peer, priv)
		if err != nil {
			return nil, nil, fmt.Errorf("the client's key: %w", err)
		}
		if shared[0] == 0 {
			clear(shared)
			continue
		}
		session, err := transport.NewAES(shared)
		clear(shared)
		if err != nil {
			return nil, nil, err
		}
		return public, session, nil
	}
	return nil, nil, fmt.Errorf("no DH private key in %d draws gives a full-length secret", maxDHTries)
}

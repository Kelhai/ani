package services

import (
	"crypto/mlkem"
	"crypto/rand"
	"fmt"

	"github.com/Kelhai/ani/client/storage"
	"github.com/Kelhai/ani/common"
	"github.com/cloudflare/circl/sign/mldsa/mldsa87"
	"golang.org/x/crypto/chacha20poly1305"
)

// InitSessionAsSender is called when we want to send to someone we've never
// talked to. We encapsulate to their KEM public key and generate our own
// ephemeral KEM keypair for the ongoing ratchet.
func InitSessionAsSender(
	peerIdentityPk, peerKemPk, peerKemPkSig []byte,
) (*storage.RatchetState, *common.RatchetHeader, error) {
	// verify the peer's KEM key is signed by their identity key
	idPk := new(mldsa87.PublicKey)
	if err := idPk.UnmarshalBinary(peerIdentityPk); err != nil {
		return nil, nil, fmt.Errorf("invalid peer identity key: %w", err)
	}
	if !mldsa87.Verify(idPk, peerKemPk, nil, peerKemPkSig) {
		return nil, nil, fmt.Errorf("KEM key signature invalid — possible MITM")
	}

	ek, err := mlkem.NewEncapsulationKey768(peerKemPk)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse peer KEM key: %w", err)
	}
	sharedSecret, kemCt := ek.Encapsulate()

	// our ephemeral ratchet KEM keypair
	ourKem, err := mlkem.GenerateKey768()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate ratchet KEM keypair: %w", err)
	}

	state := &storage.RatchetState{
		SendChainKey: storage.DeriveKey(sharedSecret, "send"),
		RecvChainKey: storage.DeriveKey(sharedSecret, "recv"),
		PeerKemPk:    peerKemPk,
		KemPk:        ourKem.EncapsulationKey().Bytes(),
		KemSk:        ourKem.Bytes(),
	}

	// The init header carries the KEM ciphertext so the receiver can derive
	// the same shared secret, plus our ephemeral KEM pubkey for the next
	// ratchet step.
	header := &common.RatchetHeader{
		KemCiphertext: kemCt,
		SenderKemPk:   ourKem.EncapsulationKey().Bytes(),
	}

	return state, header, nil
}

// InitSessionAsReceiver is called when we receive a message from someone we
// have no ratchet state for. We decapsulate using our KEM private key.
func InitSessionAsReceiver(
	ourKemSk []byte,
	header common.RatchetHeader,
) (*storage.RatchetState, error) {
	if len(header.KemCiphertext) == 0 {
		return nil, fmt.Errorf("missing KEM ciphertext in init header")
	}

	dk, err := mlkem.NewDecapsulationKey768(ourKemSk)
	if err != nil {
		return nil, fmt.Errorf("failed to parse our KEM secret key: %w", err)
	}
	sharedSecret, err := dk.Decapsulate(header.KemCiphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decapsulate: %w", err)
	}

	// generate our own ephemeral KEM keypair for future ratchet steps
	ourKem, err := mlkem.GenerateKey768()
	if err != nil {
		return nil, fmt.Errorf("failed to generate ratchet KEM keypair: %w", err)
	}

	return &storage.RatchetState{
		// recv/send are swapped relative to sender
		SendChainKey:  storage.DeriveKey(sharedSecret, "recv"),
		RecvChainKey:  storage.DeriveKey(sharedSecret, "send"),
		PeerKemPk:     header.SenderKemPk,
		KemPk:         ourKem.EncapsulationKey().Bytes(),
		KemSk:         ourKem.Bytes(),
		KemRatchetDue: true,
	}, nil
}

// advanceChain derives a message key and steps the chain key forward.
// chain key n+1 = HKDF(chain key n, "chain")
// message key n  = HKDF(chain key n, "message")
func advanceChain(chainKey []byte) (newChainKey, messageKey []byte) {
	newChainKey = storage.DeriveKey(chainKey, "chain")
	messageKey = storage.DeriveKey(chainKey, "message")
	return
}

// RatchetEncrypt encrypts plaintext, performing a KEM ratchet step first if
// KemRatchetDue is set. Returns ciphertext, the header to send, and a
// signature over (ciphertext || header bytes).
func RatchetEncrypt(
	state *storage.RatchetState,
	plaintext, aad []byte,
	identitySk *mldsa87.PrivateKey,
) (ciphertext []byte, hdr common.RatchetHeader, sig []byte, err error) {
	if state.KemRatchetDue {
		ek, e := mlkem.NewEncapsulationKey768(state.PeerKemPk)
		if e != nil {
			err = fmt.Errorf("failed to parse peer KEM key: %w", e)
			return
		}
		sharedSecret, kemCt := ek.Encapsulate()

		ourKem, e := mlkem.GenerateKey768()
		if e != nil {
			err = fmt.Errorf("failed to generate KEM keypair: %w", e)
			return
		}

		// inject new entropy into the send chain
		state.SendChainKey = storage.DeriveKey(
			append(state.SendChainKey, sharedSecret...),
			"ratchet",
		)
		state.KemPk = ourKem.EncapsulationKey().Bytes()
		state.KemSk = ourKem.Bytes()
		state.KemRatchetDue = false

		hdr.KemCiphertext = kemCt
		hdr.SenderKemPk = state.KemPk
	}

	newChain, msgKey := advanceChain(state.SendChainKey)
	state.SendChainKey = newChain

	ciphertext, err = sealMessage(plaintext, msgKey, aad)
	if err != nil {
		return
	}

	// sign (ciphertext || header json) so the receiver can authenticate both
	hdrBytes, e := marshalHeader(hdr)
	if e != nil {
		err = e
		return
	}
	sigPayload := append(ciphertext, hdrBytes...)
	sig, err = identitySk.Sign(rand.Reader, sigPayload, nil)
	return
}

// RatchetDecrypt verifies the signature, performs a KEM ratchet step if the
// header carries one, then decrypts. sigHdr is the header used for signature
// verification (always the original); decryptHdr may have KemCiphertext
// cleared on the first message to avoid a double-ratchet of the init secret.
func RatchetDecrypt(
	state *storage.RatchetState,
	ciphertext []byte,
	sigHdr, decryptHdr common.RatchetHeader,
	sig, aad []byte,
	senderIdentityPk *mldsa87.PublicKey,
) ([]byte, error) {
	hdrBytes, err := marshalHeader(sigHdr)
	if err != nil {
		return nil, err
	}
	sigPayload := append(ciphertext, hdrBytes...)
	if !mldsa87.Verify(senderIdentityPk, sigPayload, nil, sig) {
		return nil, fmt.Errorf("message signature verification failed")
	}

	if len(decryptHdr.KemCiphertext) > 0 {
		dk, err := mlkem.NewDecapsulationKey768(state.KemSk)
		if err != nil {
			return nil, fmt.Errorf("failed to parse our KEM secret key: %w", err)
		}
		sharedSecret, err := dk.Decapsulate(decryptHdr.KemCiphertext)
		if err != nil {
			return nil, fmt.Errorf("failed to decapsulate ratchet step: %w", err)
		}
		state.RecvChainKey = storage.DeriveKey(
			append(state.RecvChainKey, sharedSecret...),
			"ratchet",
		)
		state.PeerKemPk = decryptHdr.SenderKemPk
		state.KemRatchetDue = true
	}

	newChain, msgKey := advanceChain(state.RecvChainKey)
	state.RecvChainKey = newChain

	return openMessage(ciphertext, msgKey, aad)
}

func marshalHeader(hdr common.RatchetHeader) ([]byte, error) {
	// deterministic enough for signing: just concatenate the two optional
	// fields with a length prefix each so an empty field can't be confused
	// with a non-empty one.
	out := make([]byte, 0, 4+len(hdr.KemCiphertext)+len(hdr.SenderKemPk))
	out = appendLV(out, hdr.KemCiphertext)
	out = appendLV(out, hdr.SenderKemPk)
	return out, nil
}

func appendLV(dst, src []byte) []byte {
	l := len(src)
	dst = append(dst, byte(l>>8), byte(l))
	return append(dst, src...)
}

func sealMessage(plaintext, key, aad []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	return aead.Seal(nonce, nonce, plaintext, aad), nil
}

func openMessage(ciphertext, key, aad []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	ns := aead.NonceSize()
	if len(ciphertext) < ns {
		return nil, fmt.Errorf("ciphertext too short")
	}
	return aead.Open(nil, ciphertext[:ns], ciphertext[ns:], aad)
}

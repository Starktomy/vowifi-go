package ikev2

import (
	"crypto/ecdh"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
)

var (
	// RFC 3526 Group 14: 2048-bit MODP
	prime2048, _ = new(big.Int).SetString(
		"FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD129024E088A67CC74"+
			"020BBEA63B139B22514A08798E3404DDEF9519B3CD3A431B302B0A6DF25F1437"+
			"4FE1356D6D51C245E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7ED"+
			"EE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3DC2007CB8A163BF05"+
			"98DA48361C55D39A69163FA8FD24CF5F83655D23DCA3AD961C62F356208552BB"+
			"9ED529077096966D670C354E4ABC9804F1746C08CA18217C32905E462E36CE3B"+
			"E39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9DE2BCBF695581718"+
			"3995497CEA956AE515D2261898FA051015728E5A8AACAA68FFFFFFFFFFFFFFFF", 16)
	prime3072, _ = new(big.Int).SetString(
		"FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD129024E088A67CC74"+
			"020BBEA63B139B22514A08798E3404DDEF9519B3CD3A431B302B0A6DF25F1437"+
			"4FE1356D6D51C245E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7ED"+
			"EE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3DC2007CB8A163BF05"+
			"98DA48361C55D39A69163FA8FD24CF5F83655D23DCA3AD961C62F356208552BB"+
			"9ED529077096966D670C354E4ABC9804F1746C08CA18217C32905E462E36CE3B"+
			"E39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9DE2BCBF695581718"+
			"3995497CEA956AE515D2261898FA051015728E5A8AAAC42DAD33170D04507A33"+
			"A85521ABDF1CBA64ECFB850458DBEF0A8AEA71575D060C7DB3970F85A6E1E4C7"+
			"ABF5AE8CDB0933D71E8C94E04A25619DCEE3D2261AD2EE6BF12FFA06D98A0864"+
			"D87602733EC86A64521F2B18177B200CBBE117577A615D6C770988C0BAD946E2"+
			"08E24FA074E5AB3143DB5BFCE0FD108E4B82D120A93AD2CAFFFFFFFFFFFFFFFF", 16)
	gen2 = big.NewInt(2)
)

// dhGroup abstracts a Diffie-Hellman key exchange method.
type dhGroup interface {
	ID() uint16
	GeneratePrivateKey(random io.Reader, raw []byte) (any, error)
	PublicKey(priv any) []byte
	SharedKey(priv any, peerPubBytes []byte) ([]byte, error)
}

func getDHGroup(id uint16) (dhGroup, error) {
	switch id {
	case DHGroupCurve25519:
		return &ecdhGroup{id: id, curve: ecdh.X25519(), byteLen: 32}, nil
	case DHGroup256BitECP:
		return &ecdhGroup{id: id, curve: ecdh.P256(), byteLen: 32}, nil
	case DHGroup384BitECP:
		return &ecdhGroup{id: id, curve: ecdh.P384(), byteLen: 48}, nil
	case DHGroup521BitECP:
		return &ecdhGroup{id: id, curve: ecdh.P521(), byteLen: 66}, nil
	case DHGroup2048BitMODP:
		return &modpGroup{id: id, prime: prime2048, generator: gen2, byteLen: 256}, nil
	case DHGroup3072BitMODP:
		return &modpGroup{id: id, prime: prime3072, generator: gen2, byteLen: 384}, nil
	default:
		return nil, fmt.Errorf("unsupported DH group %d", id)
	}
}

// ecdhGroup implements dhGroup for NIST curves and Curve25519.
type ecdhGroup struct {
	id      uint16
	curve   ecdh.Curve
	byteLen int
}

func (g *ecdhGroup) ID() uint16 { return g.id }

func (g *ecdhGroup) GeneratePrivateKey(random io.Reader, raw []byte) (any, error) {
	if len(raw) > 0 {
		return g.curve.NewPrivateKey(append([]byte(nil), raw...))
	}
	return g.curve.GenerateKey(random)
}

func (g *ecdhGroup) PublicKey(privAny any) []byte {
	priv := privAny.(*ecdh.PrivateKey)
	if g.id == DHGroupCurve25519 {
		return priv.PublicKey().Bytes()
	}
	return priv.PublicKey().Bytes()[1:]
}

func (g *ecdhGroup) SharedKey(privAny any, peerPubBytes []byte) ([]byte, error) {
	priv := privAny.(*ecdh.PrivateKey)
	var peerPub *ecdh.PublicKey
	var err error
	if g.id == DHGroupCurve25519 {
		peerPub, err = g.curve.NewPublicKey(peerPubBytes)
	} else {
		if len(peerPubBytes) != 2*g.byteLen {
			return nil, fmt.Errorf("invalid ECDH public value length %d", len(peerPubBytes))
		}
		uncompressed := append([]byte{0x04}, peerPubBytes...)
		peerPub, err = g.curve.NewPublicKey(uncompressed)
	}
	if err != nil {
		return nil, fmt.Errorf("invalid ECDH public value: %w", err)
	}
	return priv.ECDH(peerPub)
}

// modpGroup implements dhGroup for RFC 3526 MODP groups.
type modpGroup struct {
	id        uint16
	prime     *big.Int
	generator *big.Int
	byteLen   int
}

func (g *modpGroup) ID() uint16 { return g.id }

func (g *modpGroup) GeneratePrivateKey(random io.Reader, raw []byte) (any, error) {
	if len(raw) > 0 {
		return new(big.Int).SetBytes(raw), nil
	}
	max := new(big.Int).Sub(g.prime, big.NewInt(2))
	priv, err := rand.Int(random, max)
	if err != nil {
		return nil, err
	}
	return new(big.Int).Add(priv, big.NewInt(2)), nil
}

func (g *modpGroup) PublicKey(privAny any) []byte {
	priv := privAny.(*big.Int)
	pub := new(big.Int).Exp(g.generator, priv, g.prime).Bytes()
	out := make([]byte, g.byteLen)
	copy(out[g.byteLen-len(pub):], pub)
	return out
}

func (g *modpGroup) SharedKey(privAny any, peerPubBytes []byte) ([]byte, error) {
	priv := privAny.(*big.Int)
	peer := new(big.Int).SetBytes(peerPubBytes)
	one := big.NewInt(1)
	pMinus1 := new(big.Int).Sub(g.prime, one)
	if peer.Cmp(one) <= 0 || peer.Cmp(pMinus1) >= 0 {
		return nil, errors.New("invalid DH public value")
	}
	shared := new(big.Int).Exp(peer, priv, g.prime).Bytes()
	out := make([]byte, g.byteLen)
	copy(out[g.byteLen-len(shared):], shared)
	return out, nil
}

func selectedDHGroup(sa SecurityAssociation) uint16 {
	for _, p := range sa.Proposals {
		for _, tr := range p.Transforms {
			if tr.Type == TransformDHRGroup {
				return tr.ID
			}
		}
	}
	return DHGroupCurve25519
}

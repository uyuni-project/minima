// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package packet

import (
	"crypto"
	"crypto/dsa"
	"crypto/md5"
	"crypto/rsa"
	"encoding/binary"
	"fmt"
	"hash"
	"io"
	"math/big"
	"strconv"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp/errors"
	"github.com/ProtonMail/go-crypto/openpgp/internal/encoding"
)

// PublicKeyV3 represents older, version 3 public keys. These keys are less secure and
// should not be used for signing or encrypting. They are supported here only for
// parsing version 3 key material and validating signatures.
// See RFC 4880, section 5.5.2.
type PublicKeyV3 struct {
	CreationTime time.Time
	DaysToExpire uint16
	PubKeyAlgo   PublicKeyAlgorithm
	PublicKey    *rsa.PublicKey
	Fingerprint  [16]byte
	KeyId        uint64
	IsSubkey     bool

	n, e encoding.Field
}

// newRSAPublicKeyV3 returns a PublicKey that wraps the given rsa.PublicKey.
// Included here for testing purposes only. RFC 4880, section 5.5.2:
// "an implementation MUST NOT generate a V3 key, but MAY accept it."
func newRSAPublicKeyV3(creationTime time.Time, pub *rsa.PublicKey) *PublicKeyV3 {
	pk := &PublicKeyV3{
		CreationTime: creationTime,
		PublicKey:    pub,
		n:            new(encoding.MPI).SetBig(pub.N),
		e:            new(encoding.MPI).SetBig(big.NewInt(int64(pub.E))),
	}

	pk.setFingerPrintAndKeyId()
	return pk
}

func (pk *PublicKeyV3) parse(r io.Reader) (err error) {
	// RFC 4880, section 5.5.2
	var buf [8]byte
	if _, err = readFull(r, buf[:]); err != nil {
		return
	}
	if buf[0] < 2 || buf[0] > 3 {
		return errors.UnsupportedError("public key version")
	}
	pk.CreationTime = time.Unix(int64(uint32(buf[1])<<24|uint32(buf[2])<<16|uint32(buf[3])<<8|uint32(buf[4])), 0)
	pk.DaysToExpire = binary.BigEndian.Uint16(buf[5:7])
	pk.PubKeyAlgo = PublicKeyAlgorithm(buf[7])
	switch pk.PubKeyAlgo {
	case PubKeyAlgoRSA, PubKeyAlgoRSAEncryptOnly, PubKeyAlgoRSASignOnly:
		err = pk.parseRSA(r)
	default:
		err = errors.UnsupportedError("public key type: " + strconv.Itoa(int(pk.PubKeyAlgo)))
	}
	if err != nil {
		return
	}

	pk.setFingerPrintAndKeyId()
	return
}

func (pk *PublicKeyV3) setFingerPrintAndKeyId() {
	// RFC 4880, section 12.2
	fingerPrint := md5.New()
	fingerPrint.Write(pk.n.Bytes())
	fingerPrint.Write(pk.e.Bytes())
	fingerPrint.Sum(pk.Fingerprint[:0])
	pk.KeyId = binary.BigEndian.Uint64(pk.n.Bytes()[len(pk.n.Bytes())-8:])
}

// parseRSA parses RSA public key material from the given Reader. See RFC 4880,
// section 5.5.2.
func (pk *PublicKeyV3) parseRSA(r io.Reader) (err error) {
	pk.n = new(encoding.MPI)
	if _, err = pk.n.ReadFrom(r); err != nil {
		return
	}
	pk.e = new(encoding.MPI)
	if _, err = pk.e.ReadFrom(r); err != nil {
		return
	}

	// RFC 4880 Section 12.2 requires the low 8 bytes of the
	// modulus to form the key id.
	if len(pk.n.Bytes()) < 8 {
		return errors.StructuralError("v3 public key modulus is too short")
	}
	if len(pk.e.Bytes()) > 3 {
		err = errors.UnsupportedError("large public exponent")
		return
	}
	rsa := &rsa.PublicKey{N: new(big.Int).SetBytes(pk.n.Bytes())}
	for i := 0; i < len(pk.e.Bytes()); i++ {
		rsa.E <<= 8
		rsa.E |= int(pk.e.Bytes()[i])
	}
	pk.PublicKey = rsa
	return
}

// SerializeForHash serializes the PublicKey to w with the special packet
// header format needed for hashing.
func (pk *PublicKeyV3) SerializeForHash(w io.Writer) error {
	if err := pk.SerializeSignaturePrefix(w); err != nil {
		return err
	}
	return pk.serializeWithoutHeaders(w)
}

// SerializeSignaturePrefix writes the prefix for this public key to the given Writer.
// The prefix is used when calculating a signature over this public key. See
// RFC 4880, section 5.2.4.
func (pk *PublicKeyV3) SerializeSignaturePrefix(w io.Writer) error {
	var pLength uint16
	switch pk.PubKeyAlgo {
	case PubKeyAlgoRSA, PubKeyAlgoRSAEncryptOnly, PubKeyAlgoRSASignOnly:
		pLength += pk.n.EncodedLength()
		pLength += pk.e.EncodedLength()
	default:
		return fmt.Errorf("unknown public key algorithm")
	}
	pLength += 6
	_, err := w.Write([]byte{0x99, byte(pLength >> 8), byte(pLength)})
	return err
}

func (pk *PublicKeyV3) Serialize(w io.Writer) (err error) {
	length := 8 // 8 byte header

	switch pk.PubKeyAlgo {
	case PubKeyAlgoRSA, PubKeyAlgoRSAEncryptOnly, PubKeyAlgoRSASignOnly:
		length += int(pk.n.EncodedLength())
		length += int(pk.e.EncodedLength())
	default:
		panic("unknown public key algorithm")
	}

	packetType := packetTypePublicKey
	if pk.IsSubkey {
		packetType = packetTypePublicSubkey
	}
	if err = serializeHeader(w, packetType, length); err != nil {
		return
	}
	return pk.serializeWithoutHeaders(w)
}

// serializeWithoutHeaders marshals the PublicKey to w in the form of an
// OpenPGP public key packet, not including the packet header.
func (pk *PublicKeyV3) serializeWithoutHeaders(w io.Writer) (err error) {
	var buf [8]byte
	// Version 3
	buf[0] = 3
	// Creation time
	t := uint32(pk.CreationTime.Unix())
	buf[1] = byte(t >> 24)
	buf[2] = byte(t >> 16)
	buf[3] = byte(t >> 8)
	buf[4] = byte(t)
	// Days to expire
	buf[5] = byte(pk.DaysToExpire >> 8)
	buf[6] = byte(pk.DaysToExpire)
	// Public key algorithm
	buf[7] = byte(pk.PubKeyAlgo)

	if _, err = w.Write(buf[:]); err != nil {
		return
	}

	switch pk.PubKeyAlgo {
	case PubKeyAlgoRSA, PubKeyAlgoRSAEncryptOnly, PubKeyAlgoRSASignOnly:
		if _, err = w.Write(pk.n.EncodedBytes()); err != nil {
			return
		}
		_, err = w.Write(pk.e.EncodedBytes())
		return
	}
	return errors.InvalidArgumentError("bad public-key algorithm")
}

// CanSign returns true iff this public key can generate signatures
func (pk *PublicKeyV3) CanSign() bool {
	return pk.PubKeyAlgo != PubKeyAlgoRSAEncryptOnly
}

// VerifyHashTagV3 returns nil iff sig appears to be a plausible signature over the data
// hashed into signed, based solely on its HashTag. signed is mutated by this call.
func VerifyHashTagV3(signed hash.Hash, sig *SignatureV3) (err error) {
	suffix := make([]byte, 5)
	suffix[0] = byte(sig.SigType)
	binary.BigEndian.PutUint32(suffix[1:], uint32(sig.CreationTime.Unix()))
	signed.Write(suffix)
	hashBytes := signed.Sum(nil)

	if hashBytes[0] != sig.HashTag[0] || hashBytes[1] != sig.HashTag[1] {
		return errors.SignatureError("hash tag doesn't match")
	}
	return nil
}

// VerifySignatureV3 returns nil iff sig is a valid signature, made by this _v4_
// public key, of the data hashed into signed. signed is mutated by this call.
func (pk *PublicKey) VerifySignatureV3(signed hash.Hash, sig *SignatureV3) (err error) {
	if !pk.CanSign() {
		return errors.InvalidArgumentError("public key cannot generate signatures")
	}

	suffix := make([]byte, 5)
	suffix[0] = byte(sig.SigType)
	binary.BigEndian.PutUint32(suffix[1:], uint32(sig.CreationTime.Unix()))
	signed.Write(suffix)
	hashBytes := signed.Sum(nil)

	if hashBytes[0] != sig.HashTag[0] || hashBytes[1] != sig.HashTag[1] {
		return errors.SignatureError("hash tag doesn't match")
	}

	if pk.PubKeyAlgo != sig.PubKeyAlgo {
		return errors.InvalidArgumentError("public key and signature use different algorithms")
	}

	switch pk.PubKeyAlgo {
	case PubKeyAlgoRSA, PubKeyAlgoRSASignOnly:
		rsaPublicKey := pk.PublicKey.(*rsa.PublicKey)
		if err = rsa.VerifyPKCS1v15(rsaPublicKey, sig.Hash, hashBytes, padToKeySize(rsaPublicKey, sig.RSASignature.Bytes())); err != nil {
			return errors.SignatureError("RSA verification failure")
		}
		return
	case PubKeyAlgoDSA:
		dsaPublicKey := pk.PublicKey.(*dsa.PublicKey)
		// Need to truncate hashBytes to match FIPS 186-3 section 4.6.
		subgroupSize := (dsaPublicKey.Q.BitLen() + 7) / 8
		if len(hashBytes) > subgroupSize {
			hashBytes = hashBytes[:subgroupSize]
		}
		if !dsa.Verify(dsaPublicKey, hashBytes, new(big.Int).SetBytes(sig.DSASigR.Bytes()), new(big.Int).SetBytes(sig.DSASigS.Bytes())) {
			return errors.SignatureError("DSA verification failure")
		}
		return nil
	default:
		panic("shouldn't happen")
	}
}

// VerifySignatureV3 returns nil iff sig is a valid signature, made by this
// public key, of the data hashed into signed. signed is mutated by this call.
func (pk *PublicKeyV3) VerifySignatureV3(signed hash.Hash, sig *SignatureV3) (err error) {
	if !pk.CanSign() {
		return errors.InvalidArgumentError("public key cannot generate signatures")
	}

	suffix := make([]byte, 5)
	suffix[0] = byte(sig.SigType)
	binary.BigEndian.PutUint32(suffix[1:], uint32(sig.CreationTime.Unix()))
	signed.Write(suffix)
	hashBytes := signed.Sum(nil)

	if hashBytes[0] != sig.HashTag[0] || hashBytes[1] != sig.HashTag[1] {
		return errors.SignatureError("hash tag doesn't match")
	}

	if pk.PubKeyAlgo != sig.PubKeyAlgo {
		return errors.InvalidArgumentError("public key and signature use different algorithms")
	}

	switch pk.PubKeyAlgo {
	case PubKeyAlgoRSA, PubKeyAlgoRSASignOnly:
		if err = rsa.VerifyPKCS1v15(pk.PublicKey, sig.Hash, hashBytes, sig.RSASignature.Bytes()); err != nil {
			return errors.SignatureError("RSA verification failure")
		}
		return
	default:
		// V3 public keys only support RSA.
		panic("shouldn't happen")
	}
}

// VerifyUserIdHashTagV3 returns nil iff sig appears to be a plausible signature over this _v4_
// primary key and id, based solely on its HashTag.
func (pk *PublicKey) VerifyUserIdHashTagV3(id string, sig *SignatureV3) (err error) {
	h, err := userIdSignatureV3Hash(id, pk, sig.Hash)
	if err != nil {
		return err
	}
	return VerifyHashTagV3(h, sig)
}

// VerifyUserIdHashTagV3 returns nil iff sig appears to be a plausible signature over this
// primary key and id, based solely on its HashTag.
func (pk *PublicKeyV3) VerifyUserIdHashTagV3(id string, sig *SignatureV3) (err error) {
	h, err := userIdSignatureV3Hash(id, pk, sig.Hash)
	if err != nil {
		return err
	}
	return VerifyHashTagV3(h, sig)
}

// VerifyUserIdSignatureV3 returns nil iff sig is a valid signature, made by this _v4_
// public key, that id is the identity of pub.
func (pk *PublicKey) VerifyUserIdSignatureV3(id string, pub *PublicKey, sig *SignatureV3) (err error) {
	h, err := userIdSignatureV3Hash(id, pub, sig.Hash)
	if err != nil {
		return err
	}
	return pk.VerifySignatureV3(h, sig)
}

// VerifyUserIdSignatureV3 returns nil iff sig is a valid signature, made by this
// public key, that id is the identity of pub.
func (pk *PublicKeyV3) VerifyUserIdSignatureV3(id string, pub *PublicKeyV3, sig *SignatureV3) (err error) {
	h, err := userIdSignatureV3Hash(id, pub, sig.Hash)
	if err != nil {
		return err
	}
	return pk.VerifySignatureV3(h, sig)
}

// VerifyKeyHashTagV3 returns nil iff sig appears to be a plausible signature over this _v4_
// primary key and subkey, based solely on its HashTag.
func (pk *PublicKey) VerifyKeyHashTagV3(signed *PublicKey, sig *SignatureV3) error {
	preparedHash, err := sig.PrepareVerify()
	if err != nil {
		return err
	}
	h, err := keySignatureHash(pk, signed, preparedHash)
	if err != nil {
		return err
	}
	return VerifyHashTagV3(h, sig)
}

// VerifyKeyHashTagV3 returns nil iff sig appears to be a plausible signature over this
// primary key and subkey, based solely on its HashTag.
func (pk *PublicKeyV3) VerifyKeyHashTagV3(signed *PublicKeyV3, sig *SignatureV3) error {
	preparedHash, err := sig.PrepareVerify()
	if err != nil {
		return err
	}
	h, err := keySignatureHash(pk, signed, preparedHash)
	if err != nil {
		return err
	}
	return VerifyHashTagV3(h, sig)
}

// VerifyKeySignatureV3 returns nil iff sig is a valid signature, made by this _v4_
// public key, of signed.
func (pk *PublicKey) VerifyKeySignatureV3(signed *PublicKey, sig *SignatureV3) (err error) {
	if signed.CanSign() {
		// Signing subkeys must be cross-signed, and this is not supported for v3 sigs
		return errors.StructuralError("signing subkey may not have a v3 binding signature")
	}
	preparedHash, err := sig.PrepareVerify()
	if err != nil {
		return err
	}
	h, err := keySignatureHash(pk, signed, preparedHash)
	if err != nil {
		return err
	}
	return pk.VerifySignatureV3(h, sig)
}

// VerifyKeySignatureV3 returns nil iff sig is a valid signature, made by this
// public key, of signed.
func (pk *PublicKeyV3) VerifyKeySignatureV3(signed *PublicKeyV3, sig *SignatureV3) (err error) {
	preparedHash, err := sig.PrepareVerify()
	if err != nil {
		return err
	}
	h, err := keySignatureHash(pk, signed, preparedHash)
	if err != nil {
		return err
	}
	return pk.VerifySignatureV3(h, sig)
}

// VerifyRevocationHashTagV3 returns nil iff sig appears to be a plausible signature over this _v4_
// key, based solely on its HashTag.
func (pk *PublicKey) VerifyRevocationHashTagV3(sig *SignatureV3) (err error) {
	preparedHash, err := sig.PrepareVerify()
	if err != nil {
		return err
	}
	err = keyRevocationHash(pk, preparedHash)
	if err != nil {
		return err
	}
	return VerifyHashTagV3(preparedHash, sig)
}

// VerifyRevocationHashTagV3 returns nil iff sig appears to be a plausible signature over this
// key, based solely on its HashTag.
func (pk *PublicKeyV3) VerifyRevocationHashTagV3(sig *SignatureV3) (err error) {
	preparedHash, err := sig.PrepareVerify()
	if err != nil {
		return err
	}
	err = keyRevocationHash(pk, preparedHash)
	if err != nil {
		return err
	}
	return VerifyHashTagV3(preparedHash, sig)
}

// VerifyRevocationSignatureV3 returns nil iff sig is a valid signature, made by this _v4_
// public key.
func (pk *PublicKey) VerifyRevocationSignatureV3(sig *SignatureV3) (err error) {
	preparedHash, err := sig.PrepareVerify()
	if err != nil {
		return err
	}
	err = keyRevocationHash(pk, preparedHash)
	if err != nil {
		return err
	}
	return pk.VerifySignatureV3(preparedHash, sig)
}

// VerifyRevocationSignatureV3 returns nil iff sig is a valid signature, made by this
// public key.
func (pk *PublicKeyV3) VerifyRevocationSignatureV3(sig *SignatureV3) (err error) {
	preparedHash, err := sig.PrepareVerify()
	if err != nil {
		return err
	}
	err = keyRevocationHash(pk, preparedHash)
	if err != nil {
		return err
	}
	return pk.VerifySignatureV3(preparedHash, sig)
}

// userIdSignatureV3Hash returns a Hash of the message that needs to be signed
// to assert that pk is a valid key for id.
func userIdSignatureV3Hash(id string, pk signingKey, hfn crypto.Hash) (h hash.Hash, err error) {
	if !hfn.Available() {
		return nil, errors.UnsupportedError("hash function")
	}
	h = hfn.New()

	// RFC 4880, section 5.2.4
	err = pk.SerializeForHash(h)

	h.Write([]byte(id))

	return
}

// KeyIdString returns the public key's fingerprint in capital hex
// (e.g. "6C7EE1B8621CC013").
func (pk *PublicKeyV3) KeyIdString() string {
	return fmt.Sprintf("%X", pk.KeyId)
}

// KeyIdShortString returns the short form of public key's fingerprint
// in capital hex, as shown by gpg --list-keys (e.g. "621CC013").
func (pk *PublicKeyV3) KeyIdShortString() string {
	return fmt.Sprintf("%X", pk.KeyId&0xFFFFFFFF)
}

// BitLength returns the bit length for the given public key.
func (pk *PublicKeyV3) BitLength() (bitLength uint16, err error) {
	switch pk.PubKeyAlgo {
	case PubKeyAlgoRSA, PubKeyAlgoRSAEncryptOnly, PubKeyAlgoRSASignOnly:
		bitLength = pk.n.BitLength()
	default:
		err = errors.InvalidArgumentError("bad public-key algorithm")
	}
	return
}

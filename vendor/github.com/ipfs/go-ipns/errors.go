package ipns

import (
	"errors"
)

// ErrExpiredRecord should be returned when an ipns record is
// invalid due to being too old
//
// Deprecated: use github.com/ipfs/boxo/ipns.ErrExpiredRecord
var ErrExpiredRecord = errors.New("expired record")

// ErrUnrecognizedValidity is returned when an IpnsRecord has an
// unknown validity type.
//
// Deprecated: use github.com/ipfs/boxo/ipns.ErrUnrecognizedValidity
var ErrUnrecognizedValidity = errors.New("unrecognized validity type")

// ErrInvalidPath should be returned when an ipns record path
// is not in a valid format
//
// Deprecated: use github.com/ipfs/boxo/ipns.ErrInvalidPath
var ErrInvalidPath = errors.New("record path invalid")

// ErrSignature should be returned when an ipns record fails
// signature verification
//
// Deprecated: use github.com/ipfs/boxo/ipns.ErrSignature
var ErrSignature = errors.New("record signature verification failed")

// ErrKeyFormat should be returned when an ipns record key is
// incorrectly formatted (not a peer ID)
//
// Deprecated: use github.com/ipfs/boxo/ipns.ErrKeyFormat
var ErrKeyFormat = errors.New("record key could not be parsed into peer ID")

// ErrPublicKeyNotFound should be returned when the public key
// corresponding to the ipns record path cannot be retrieved
// from the peer store
//
// Deprecated: use github.com/ipfs/boxo/ipns.ErrPublicKeyNotFound
var ErrPublicKeyNotFound = errors.New("public key not found in peer store")

// ErrPublicKeyMismatch should be returned when the public key embedded in the
// record doesn't match the expected public key.
//
// Deprecated: use github.com/ipfs/boxo/ipns.ErrPublicKeyMismatch
var ErrPublicKeyMismatch = errors.New("public key in record did not match expected pubkey")

// ErrBadRecord should be returned when an ipns record cannot be unmarshalled
//
// Deprecated: use github.com/ipfs/boxo/ipns.ErrBadRecord
var ErrBadRecord = errors.New("record could not be unmarshalled")

// 10 KiB limit defined in https://github.com/ipfs/specs/pull/319
//
// Deprecated: use github.com/ipfs/boxo/ipns.MaxRecordSize
const MaxRecordSize int = 10 << (10 * 1)

// ErrRecordSize should be returned when an ipns record is
// invalid due to being too big
//
// Deprecated: use github.com/ipfs/boxo/ipns.ErrRecordSize
var ErrRecordSize = errors.New("record exceeds allowed size limit")

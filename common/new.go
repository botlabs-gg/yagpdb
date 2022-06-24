// This file contains helpers to obtain pointers to builtin types.

package common

// NewBool returns a pointer to the given bool value.
func NewBool(b bool) *bool { return &b }

// NewString returns a pointer to the given string value.
func NewString(s string) *string { return &s }

// NewInt returns a pointer to the given int value.
func NewInt(i int) *int { return &i }

// NewInt8 returns a pointer to the given int8 value.
func NewInt8(i int8) *int8 { return &i }

// NewInt16 returns a pointer to the given int16 value.
func NewInt16(i int16) *int16 { return &i }

// NewInt32 returns a pointer to the given int32 value.
func NewInt32(i int32) *int32 { return &i }

// NewInt64 returns a pointer to the given int64 value.
func NewInt64(i int64) *int64 { return &i }

// NewUint returns a pointer to the given uint value.
func NewUint(i uint) *uint { return &i }

// NewUint8 returns a pointer to the given uint8 value.
func NewUint8(i uint8) *uint8 { return &i }

// NewUint16 returns a pointer to the given uint16 value.
func NewUint16(i uint16) *uint16 { return &i }

// NewUint32 returns a pointer to the given uint32 value.
func NewUint32(i uint32) *uint32 { return &i }

// NewUint64 returns a pointer to the given uint64 value.
func NewUint64(i uint64) *uint64 { return &i }

// NewFloat32 returns a pointer to the given float32 value.
func NewFloat32(f float32) *float32 { return &f }

// NewFloat64 returns a pointer to the given float64 value.
func NewFloat64(f float64) *float64 { return &f }

// NewByte returns a pointer to the given byte value.
func NewByte(b byte) *byte { return &b }

// NewRune returns a pointer to the given rune value.
func NewRune(r rune) *rune { return &r }

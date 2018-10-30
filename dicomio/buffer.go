// Package dicomio provides utility functions for encoding and
// decoding low-level DICOM data types, such as integers and strings.
package dicomio

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"golang.org/x/text/encoding"
)

// NativeByteOrder is the byte order of this machine.
//
// TODO(saito) Auto-detect the native byte order. For now, to assume that every
//machine in the world is little endian is not too horrible.
var NativeByteOrder = binary.LittleEndian

type transferSyntaxStackEntry struct {
	bo       binary.ByteOrder
	implicit IsImplicitVR
}

type stackEntry struct {
	limit int64
	err   error
}

// Encoder is a helper class for encoding low-level DICOM data types.
type Encoder struct {
	err error

	out io.Writer
	bo  binary.ByteOrder
	// "implicit" isn't used by Encoder internally. It's there for the user
	// of Encoder to see the current transfer syntax.
	implicit IsImplicitVR

	// Stack of old transfer syntaxes. Used by {Push,Pop}TransferSyntax.
	oldTransferSyntaxes []transferSyntaxStackEntry
}

// NewBytesEncoder creates a new Encoder that writes to an in-memory buffer. The
// contents can be obtained via Bytes() method.
func NewBytesEncoder(bo binary.ByteOrder, implicit IsImplicitVR) *Encoder {
	return &Encoder{
		err:      nil,
		out:      &bytes.Buffer{},
		bo:       bo,
		implicit: implicit,
	}
}

// NewBytesEncoderWithTransferSyntax is similar to NewBytesEncoder, but it takes
// a transfersyntaxuid.
func NewBytesEncoderWithTransferSyntax(transferSyntaxUID string) *Encoder {
	endian, implicit, err := ParseTransferSyntaxUID(transferSyntaxUID)
	if err == nil {
		return NewBytesEncoder(endian, implicit)
	}
	e := NewBytesEncoder(binary.LittleEndian, ExplicitVR)
	e.SetErrorf("%v: Unknown transfer syntax uid", transferSyntaxUID)
	return e
}

// NewEncoderWithTransferSyntax is similar to NewEncoder, but it takes a
// transfersyntaxuid.
func NewEncoderWithTransferSyntax(out io.Writer, transferSyntaxUID string) *Encoder {
	endian, implicit, err := ParseTransferSyntaxUID(transferSyntaxUID)
	if err == nil {
		return NewEncoder(out, endian, implicit)
	}
	e := NewEncoder(out, binary.LittleEndian, ExplicitVR)
	e.SetErrorf("%v: Unknown transfer syntax uid", transferSyntaxUID)
	return e
}

// NewEncoder creates a new encoder that writes to "out".
func NewEncoder(out io.Writer, bo binary.ByteOrder, implicit IsImplicitVR) *Encoder {
	return &Encoder{
		err:      nil,
		out:      out,
		bo:       bo,
		implicit: implicit,
	}
}

// TransferSyntax returns the current transfer syntax.
func (e *Encoder) TransferSyntax() (binary.ByteOrder, IsImplicitVR) {
	return e.bo, e.implicit
}

// PushTransferSyntax temporarily changes the encoding
// format. PopTrasnferSyntax() will restore the old format.
func (e *Encoder) PushTransferSyntax(bo binary.ByteOrder, implicit IsImplicitVR) {
	e.oldTransferSyntaxes = append(e.oldTransferSyntaxes,
		transferSyntaxStackEntry{e.bo, e.implicit})
	e.bo = bo
	e.implicit = implicit
}

// PopTransferSyntax restores the encoding format active before the last call to
// PushTransferSyntax().
func (e *Encoder) PopTransferSyntax() {
	ts := e.oldTransferSyntaxes[len(e.oldTransferSyntaxes)-1]
	e.bo = ts.bo
	e.implicit = ts.implicit
	e.oldTransferSyntaxes = e.oldTransferSyntaxes[:len(e.oldTransferSyntaxes)-1]
}

// SetError sets the error to be reported by future Error() calls.  If called
// multiple times with different errors, Error() will return one of them, but
// exactly which is unspecified.
//
// REQUIRES: err != nil
func (e *Encoder) SetError(err error) {
	if err != nil && e.err == nil {
		e.err = err
	}
}

// SetErrorf is similar to SetError, but takes a printf format string.
func (e *Encoder) SetErrorf(format string, args ...interface{}) {
	e.SetError(fmt.Errorf(format, args...))
}

// Error returns an error set by SetError(), if any.  Returns nil if SetError()
// has never been called.
func (e *Encoder) Error() error { return e.err }

// Bytes returns the encoded data.
//
// REQUIRES: Encoder was created by NewBytesEncoder (not NewEncoder).
// REQUIRES: e.Error() == nil.
func (e *Encoder) Bytes() []byte {
	doassert(len(e.oldTransferSyntaxes) == 0)
	if e.err != nil {
		panic(e.err)
	}
	return e.out.(*bytes.Buffer).Bytes()
}

func (e *Encoder) WriteByte(v byte) {
	if err := binary.Write(e.out, e.bo, &v); err != nil {
		e.SetError(err)
	}
}

func (e *Encoder) WriteUInt16(v uint16) {
	if err := binary.Write(e.out, e.bo, &v); err != nil {
		e.SetError(err)
	}
}

func (e *Encoder) WriteUInt32(v uint32) {
	if err := binary.Write(e.out, e.bo, &v); err != nil {
		e.SetError(err)
	}
}

func (e *Encoder) WriteInt16(v int16) {
	if err := binary.Write(e.out, e.bo, &v); err != nil {
		e.SetError(err)
	}
}

func (e *Encoder) WriteInt32(v int32) {
	if err := binary.Write(e.out, e.bo, &v); err != nil {
		e.SetError(err)
	}
}

func (e *Encoder) WriteFloat32(v float32) {
	if err := binary.Write(e.out, e.bo, &v); err != nil {
		e.SetError(err)
	}
}

func (e *Encoder) WriteFloat64(v float64) {
	if err := binary.Write(e.out, e.bo, &v); err != nil {
		e.SetError(err)
	}
}

// WriteString writes the string, withoutout any length prefix or padding.
func (e *Encoder) WriteString(v string) {
	if _, err := e.out.Write([]byte(v)); err != nil {
		e.SetError(err)
	}
}

// WriteZeros encodes an array of zero bytes.
func (e *Encoder) WriteZeros(len int) {
	// TODO(saito) reuse the buffer!
	zeros := make([]byte, len)
	e.out.Write(zeros)
}

// Copy the given data to the output.
func (e *Encoder) WriteBytes(v []byte) {
	e.out.Write(v)
}

// IsImplicitVR defines whether a 2-character VR tag is emit with each data
// element.
type IsImplicitVR int

const (
	// TODO(saito) Where are implicit/explicit defined? Add a ref!

	// ImplicitVR encodes a data element without a VR tag. The reader
	// consults the static tag->VR mapping (see tags.go) defined by DICOM
	// standard.
	ImplicitVR IsImplicitVR = iota

	// ExplicitVR stores the 2-byte VR value inline w/ a data element.
	ExplicitVR

	// UnknownVR is to be used when you never encode or decode DataElement.
	UnknownVR
)

// Decoder is a helper class for decoder low-level DICOM data types.
type Decoder struct {
	in  *bufio.Reader
	err error
	bo  binary.ByteOrder
	// "implicit" isn't used by Decoder internally. It's there for the user
	// of Decoder to see the current transfer syntax.
	implicit IsImplicitVR
	// Max bytes to read from "in".
	limit int64
	// Cumulative # bytes read.
	pos int64

	// For decoding raw strings in DICOM file into utf-8.
	// If nil, assume ASCII. Cf P3.5 6.1.2.1
	codingSystem CodingSystem

	// Stack of old transfer syntaxes. Used by {Push,Pop}TransferSyntax.
	oldTransferSyntaxes []transferSyntaxStackEntry
	// Stack of old limits. Used by {Push,Pop}Limit.
	// INVARIANT: oldLimits[] store values in decreasing order.
	stateStack []stackEntry
}

// NewDecoder creates a decoder object that reads up to "limit" bytes from "in".
// Don't pass just an arbitrary large number as the "limit". The underlying code
// assumes that "limit" accurately bounds the end of the data.
func NewDecoder(
	in io.Reader,
	bo binary.ByteOrder,
	implicit IsImplicitVR) *Decoder {
	return &Decoder{
		in:       bufio.NewReader(in),
		err:      nil,
		bo:       bo,
		implicit: implicit,
		pos:      0,
		limit:    math.MaxInt64,
	}
}

// NewBytesDecoder creates a decoder that reads from a sequence of bytes. See
// NewDecoder() for explanation of other parameters.
func NewBytesDecoder(data []byte, bo binary.ByteOrder, implicit IsImplicitVR) *Decoder {
	return NewDecoder(bytes.NewReader(data), bo, implicit)
}

// NewBytesDecoderWithTransferSyntax is similar to NewBytesDecoder, but it takes
// a transfer syntax UID instead of a <byteorder, IsImplicitVR> pair.
func NewBytesDecoderWithTransferSyntax(data []byte, transferSyntaxUID string) *Decoder {
	endian, implicit, err := ParseTransferSyntaxUID(transferSyntaxUID)
	if err == nil {
		return NewBytesDecoder(data, endian, implicit)
	}
	d := NewBytesDecoder(data, binary.LittleEndian, ExplicitVR)
	d.SetError(fmt.Errorf("%v: Unknown transfer syntax uid", transferSyntaxUID))
	return d
}

// SetError sets the error to be reported by future Error() or Finish() calls.
//
// REQUIRES: err != nil
func (d *Decoder) SetError(err error) {
	if err != nil && d.err == nil {
		if err != io.EOF {
			err = fmt.Errorf("%s (file offset %d)", err.Error(), d.pos)
		}
		d.err = err
	}
}

// SetErrorf is similar to SetError, but takes a printf format string.
func (d *Decoder) SetErrorf(format string, args ...interface{}) {
	d.SetError(fmt.Errorf(format, args...))
}

// TransferSyntax returns the current transfer syntax.
func (d *Decoder) TransferSyntax() (bo binary.ByteOrder, implicit IsImplicitVR) {
	return d.bo, d.implicit
}

// PushTransferSyntax temporarily changes the encoding
// format. PopTrasnferSyntax() will restore the old format.
func (d *Decoder) PushTransferSyntax(bo binary.ByteOrder, implicit IsImplicitVR) {
	d.oldTransferSyntaxes = append(d.oldTransferSyntaxes, transferSyntaxStackEntry{d.bo, d.implicit})
	d.bo = bo
	d.implicit = implicit
}

// PushTransferSyntaxByUID is similar to PushTransferSyntax, but it takes a
// transfer syntax UID.
func (d *Decoder) PushTransferSyntaxByUID(uid string) {
	endian, implicit, err := ParseTransferSyntaxUID(uid)
	if err != nil {
		d.SetError(err)
	}
	d.PushTransferSyntax(endian, implicit)
}

// SetCodingSystem overrides the default (7bit ASCII) decoder used when
// converting a byte[] to a string.
func (d *Decoder) SetCodingSystem(cs CodingSystem) {
	d.codingSystem = cs
}

// PopTransferSyntax restores the encoding format active before the last call to
// PushTransferSyntax().
func (d *Decoder) PopTransferSyntax() {
	e := d.oldTransferSyntaxes[len(d.oldTransferSyntaxes)-1]
	d.bo = e.bo
	d.implicit = e.implicit
	d.oldTransferSyntaxes = d.oldTransferSyntaxes[:len(d.oldTransferSyntaxes)-1]
}

// PushLimit temporarily overrides the end of the buffer and clears
// d.err. PopLimit() will restore the old limit and error.
//
// REQUIRES: limit must be smaller than the current limit
func (d *Decoder) PushLimit(bytes int64) {
	newLimit := d.pos + bytes
	if newLimit > d.limit {
		d.SetError(fmt.Errorf("Trying to read %d bytes beyond buffer end", newLimit-d.limit))
		newLimit = d.pos
	}
	d.stateStack = append(d.stateStack, stackEntry{limit: d.limit, err: d.err})
	d.limit = newLimit
	d.err = nil
}

// PopLimit restores the old limit overridden by PushLimit.
func (d *Decoder) PopLimit() {
	if d.pos < d.limit {
		// d.pos < d.limit iff parse error happened and the caller didn't fully
		// consume the input.  Here we skip over the unparsable part.  This is just a
		// heuristics to parse as much data as possible from corrupt files.
		d.Skip(int(d.limit - d.pos))
	}
	last := len(d.stateStack) - 1
	d.limit = d.stateStack[last].limit
	if d.stateStack[last].err != nil {
		d.err = d.stateStack[last].err
	}
	d.stateStack = d.stateStack[:last]
}

// Error returns an error encountered so far.
func (d *Decoder) Error() error { return d.err }

// Finish must be called after using the decoder. It returns any error
// encountered during decoding. It also returns an error if some data is left
// unconsumed.
func (d *Decoder) Finish() error {
	if d.err != nil {
		return d.err
	}
	if !d.EOF() {
		return fmt.Errorf("Decoder found junk")
	}
	return nil
}

// Read implements the io.Reader interface.
func (d *Decoder) Read(p []byte) (int, error) {
	desired := d.len()
	if desired == 0 {
		if len(p) == 0 {
			return 0, nil
		}
		return 0, io.EOF
	}
	if desired < int64(len(p)) {
		p = p[:desired]
	}
	n, err := d.in.Read(p)
	if n >= 0 {
		d.pos += int64(n)
	}
	return n, err
}

// EOF checks if there is any more data to read.
func (d *Decoder) EOF() bool {
	if d.err != nil {
		return true
	}
	if d.limit-d.pos <= 0 {
		return true
	}
	data, _ := d.in.Peek(1)
	return len(data) == 0
}

// BytesRead returns the cumulative # of bytes read so far.
func (d *Decoder) BytesRead() int64 { return d.pos }

// Len returns the number of bytes yet consumed.
func (d *Decoder) len() int64 {
	return d.limit - d.pos
}

// ReadByte reads a single byte from the buffer. On EOF, it returns a junk
// value, and sets an error to be returned by Error() or Finish().
func (d *Decoder) ReadByte() (v byte) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.SetError(err)
		return 0
	}
	return v
}

func (d *Decoder) ReadUInt32() (v uint32) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.SetError(err)
	}
	return v
}

func (d *Decoder) ReadInt32() (v int32) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.SetError(err)
	}
	return v
}

func (d *Decoder) ReadUInt16() (v uint16) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.SetError(err)
	}
	return v
}

func (d *Decoder) ReadInt16() (v int16) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.SetError(err)
	}
	return v
}

func (d *Decoder) ReadFloat32() (v float32) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.SetError(err)
	}
	return v
}

func (d *Decoder) ReadFloat64() (v float64) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.SetError(err)
	}
	return v
}

func internalReadString(d *Decoder, sd *encoding.Decoder, length int) string {
	bytes := d.ReadBytes(length)
	if len(bytes) == 0 {
		return ""
	}
	if sd == nil {
		// Assume that UTF-8 is a superset of ASCII.
		// TODO(saito) check that string is 7-bit clean.
		return string(bytes)
	}
	bytes, err := sd.Bytes(bytes)
	if err != nil {
		d.SetError(err)
		return ""
	}
	return string(bytes)
}

func (d *Decoder) ReadStringWithCodingSystem(csType CodingSystemType, length int) string {
	var sd *encoding.Decoder
	switch csType {
	case AlphabeticCodingSystem:
		sd = d.codingSystem.Alphabetic
	case IdeographicCodingSystem:
		sd = d.codingSystem.Ideographic
	case PhoneticCodingSystem:
		sd = d.codingSystem.Phonetic
	default:
		panic(csType)
	}
	return internalReadString(d, sd, length)
}

func (d *Decoder) ReadString(length int) string {
	return internalReadString(d, d.codingSystem.Ideographic, length)
}

func (d *Decoder) ReadBytes(length int) []byte {
	if d.len() < int64(length) {
		d.SetError(fmt.Errorf("ReadBytes: requested %d, available %d", length, d.len()))
		return nil
	}
	v := make([]byte, length)
	remaining := v
	for len(remaining) > 0 {
		n, err := d.Read(remaining)
		if err != nil {
			d.SetError(err)
			break
		}
		if n < 0 || n > len(remaining) {
			panic(fmt.Sprintf("Remaining: %d %d", n, len(remaining)))
		}
		remaining = remaining[n:]
	}
	doassert(d.err != nil || len(remaining) == 0)
	return v
}

// Skip advances the read pointer by "length" bytes.
func (d *Decoder) Skip(length int) {
	if d.len() < int64(length) {
		d.SetError(fmt.Errorf("Skip: requested %d, available %d",
			length, d.len()))
		return
	}
	junkSize := 1 << 16
	if length < junkSize {
		junkSize = length
	}
	junk := make([]byte, junkSize)
	remaining := length
	for remaining > 0 {
		tmpLength := len(junk)
		if remaining < tmpLength {
			tmpLength = remaining
		}
		tmpBuf := junk[:tmpLength]
		n, err := d.Read(tmpBuf)
		if err != nil {
			d.SetError(err)
			break
		}
		doassert(n > 0)
		remaining -= n
	}
	doassert(d.err != nil || remaining == 0)
}

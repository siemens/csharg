// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

package pcapng

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"math"
	"regexp"
	"strings"

	"github.com/thediveo/nonstd/xslog"
	"gopkg.in/yaml.v3"

	"github.com/siemens/csharg/v2/api"
)

const (
	// yamlmarker describes the "magic" signature of a capture target YAML
	// document.
	targetmarker = "---\n# capture target information\n"

	// minSHBLen is the length of an SHB without any options: block type,
	// length, byte-order magic, version, section length, trailing length.
	minSHBLen = 4 + 4 + 4 + 2 + 2 + 8 + 4
	// maxSHBLen limits how much stream data gets buffered for SHB editing.
	maxSHBLen = 1 << 20
	// maxOptionValueLen is the largest value length an option can encode.
	maxOptionValueLen = math.MaxUint16
)

var (
	// markerstart matches the first capture target YAML document.
	markerstart = regexp.MustCompile(`(?s)(^|\n)` + targetmarker)
	// markerend matches an optional YAML end/next document marker. Yes, we know
	// that is not fully correct, but must suffice for now.
	markerend = regexp.MustCompile(`(?s)\n---($|\n)`)
)

// StreamEditor allows editing the first section header block (SHB) of a pcapng
// packet capture stream.
type StreamEditor struct {
	Endian        binary.ByteOrder
	sink          io.Writer
	passThrough   bool
	shb           []byte
	shbLen        uint32
	container     *api.Target
	captureFilter string
	noProm        bool
	err           error
}

// ContainerInfo represents the container information to be added to the capture
// comments of a packet capture stream.
type ContainerInfo struct {
	ContainerName string `yaml:"container-name"`
	ContainerType string `yaml:"container-type"`
	NodeName      string `yaml:"node-name"`
	*ClusterInfo  `yaml:"cluster,omitempty"`
	CaptureFilter string `yaml:"capture-filter,omitempty"`
	NoProm        bool   `yaml:"no-promiscuous-mode,omitempty"`
}

// ClusterInfo represents the cluster information to be added to the capture
// comments of a packet capture stream.
type ClusterInfo struct {
	UID string `yaml:"uid,omitempty"`
}

// NewStreamEditor returns a new pcapng packet stream data editor, connected to
// the specified writer (which can be a pipe, file, et cetera).
func NewStreamEditor(sink io.Writer, container *api.Target, captureFilter string, noProm bool) *StreamEditor {
	if container == nil {
		container = &api.Target{}
	}
	return &StreamEditor{
		sink:          sink,
		container:     container,
		captureFilter: captureFilter,
		noProm:        noProm,
	}
}

// Write writes some octets into the pcapng stream editor which it might then
// edit if required before writing the (edited) stream to the associated writer
// sink.
//
// As the edited stream cannot be mapped back onto b, Write reports zero octets
// written when the sink fails or writes short, and then fails all further
// writes with the same error.
func (pe *StreamEditor) Write(b []byte) (int, error) {
	if pe.err != nil {
		return 0, pe.err
	}
	out := pe.process(b)
	n, err := pe.sink.Write(out)
	if err == nil && n < len(out) {
		err = io.ErrShortWrite
	}
	if err != nil {
		slog.Debug("pcapng stream broken", xslog.Error(err))
		pe.err = err
		return 0, err
	}
	// While collecting the SHB, nothing might reach the sink yet, but all of b
	// has been consumed nevertheless.
	return len(b), nil
}

// Processes a block of packet stream data, editing the first section header
// block, but not touching the packet stream data elsewhere.
func (pe *StreamEditor) process(b []byte) []byte {
	if pe.passThrough {
		return b
	}
	pe.shb = append(pe.shb, b...)
	// Do we already have enough octets from the stream to decode the
	// length of this SHB?
	if pe.shbLen == 0 && len(pe.shb) >= 12 {
		if !pe.shbLenEndianness() {
			// There's a problem with this stream, so simply switch into
			// pass-through mode without editing the SHB.
			pe.passThrough = true
			pc := pe.shb
			pe.shb = []byte{}
			return pc
		}
	}
	// Did we gather the complete SHB yet?
	if pe.shbLen != 0 && uint32(len(pe.shb)) >= pe.shbLen {
		return pe.processSHB()
	}
	// Do not return anything yet, as we're still collecting dust, erm, octets.
	return []byte{}
}

// processSHB processes the (first) Section Header Block, updating or inserting
// the comment option with capture target information.
func (pe *StreamEditor) processSHB() []byte {
	// Decode SHB information: first comes the fixed information...
	major := pe.Endian.Uint16(pe.shb[12:14])
	minor := pe.Endian.Uint16(pe.shb[14:16])
	sectionLen := pe.Endian.Uint64(pe.shb[16:24])
	slog.Debug("found section header block",
		slog.String("version", fmt.Sprintf("%d.%d", major, minor)))
	if sectionLen == ^uint64(0) {
		slog.Debug("signalled unknown section length")
	} else {
		slog.Debug("signalled overall section length",
			slog.Uint64("length", sectionLen))
	}
	// ...then comes a list of options, terminated by an end-of-options
	// option.
	offset := uint32(24)
	options := []*Option{}
	var firstComment *Option
	// Note, the block length is the total block length, including the
	// leading and trailing 32bit block length fields; it's NOT the netto
	// content.
	optsEnd := pe.shbLen - 4
	for offset < optsEnd {
		opt, skip := NewOption(pe.shb[offset:optsEnd], pe.Endian)
		if opt == nil {
			break
		}
		offset += uint32(skip)
		if opt.Code == OptComment && firstComment == nil {
			// Do not append the first comment, but store it aside for
			// modification.
			firstComment = opt
		} else {
			options = append(options, opt)
		}
		if opt.Code <= OptSHBUserAppl {
			slog.Debug("option",
				slog.Int("type", int(opt.Code)),
				slog.String("value", opt.String()))
		} else {
			slog.Debug("option", slog.Int("type", int(opt.Code)))
		}
	}
	// Edit the first comment -- or be the first to create one :p
	var comment string
	if firstComment != nil {
		slog.Debug("removing existing SHB comment with container meta information, then updating")
		comment = firstComment.String()
		if start := markerstart.FindStringIndex(comment); len(start) == 2 {
			if comment[start[0]] == '\n' {
				start[0]++
			}
			if end := markerend.FindStringIndex(comment[start[1]:]); len(end) == 2 {
				// We found an end marker, so cut out the target info YAML. But
				// since there is another YAML document following, make sure
				// there's the separator still present.
				comment = comment[:start[0]] + comment[start[0]+end[0]:]
			} else {
				// There is no end marker: in this case the target info YAML
				// runs until the end of the comment. Now just take the comment
				// part before the target info YAML.
				comment = comment[:start[0]]
			}
		}
	} else {
		slog.Debug("creating fresh SHB comment with container meta information")
	}
	targetInfo := targetmarker
	ci := ContainerInfo{
		ContainerName: pe.container.Name,
		ContainerType: pe.container.Type,
		NodeName:      pe.container.NodeName,
		CaptureFilter: pe.captureFilter,
		NoProm:        pe.noProm,
	}
	if cluster := pe.container.Cluster; cluster != nil {
		ci.ClusterInfo = &ClusterInfo{
			UID: cluster.UID,
		}
	}
	y, err := yaml.Marshal(ci)
	if err == nil {
		targetInfo += string(y)
	} else {
		slog.Error("cannot create container YAML meta data",
			xslog.Error(err))
	}
	// Append target info YAML to comment, so make sure there always is a proper
	// line break before our YAML.
	if comment != "" && !strings.HasSuffix(comment, "\n") {
		comment += "\n"
	}
	if len(comment)+len(targetInfo) > maxOptionValueLen {
		slog.Warn("existing SHB comment too long, dropping it in favor of target information")
		comment = ""
	}
	comment += targetInfo
	options = append(
		[]*Option{
			{Code: OptComment, Value: []byte(comment)}},
		options...)
	// Create new SHB...
	shbOpts := []byte{}
	for _, opt := range options {
		shbOpts = append(shbOpts, opt.Bytes(pe.Endian)...)
	}
	// ...but only now we can calculate the total length of the SHB.
	shbLen := 4 + 4 + 4 + 2 + 2 + 8 + len(shbOpts) + 4
	shb := make([]byte, shbLen)
	pe.Endian.PutUint32(shb[0:4], 0x0a0d0d0a)
	pe.Endian.PutUint32(shb[4:8], uint32(shbLen))
	pe.Endian.PutUint32(shb[8:12], 0x1a2b3c4d)
	pe.Endian.PutUint16(shb[12:14], major)
	pe.Endian.PutUint16(shb[14:16], minor)
	pe.Endian.PutUint64(shb[16:24], ^uint64(0))
	copy(shb[24:], shbOpts)
	pe.Endian.PutUint32(shb[shbLen-4:], uint32(shbLen))
	// Don't forget to add the overspill because we might have gotten
	// more bytes than just the SHB.
	shb = append(shb, pe.shb[pe.shbLen:]...)
	// We're done and now enter pass-through mode.
	pe.passThrough = true
	pe.shb = []byte{}
	return shb
}

// shbLenEndianness detects the endianness as well as the length of a
// section header block; for this, the first 12 octets are needed.
func (pe *StreamEditor) shbLenEndianness() bool {
	// This is the first time that we received enough pcapng data to find out
	// how long the SHB is going to be: the SHB begins with its block type,
	// followed by the block total length, and -- most importantly -- the
	// "byte-order magic" ... which tells us the endianness of values, such
	// as the block length.
	if !bytes.Equal(pe.shb[0:4], []byte{0x0a, 0x0d, 0x0d, 0x0a}) {
		slog.Error("invalid packet capture stream; must begin with section header block")
		return false
	}
	if bytes.Equal(pe.shb[8:12], []byte{0x1a, 0x2b, 0x3c, 0x4d}) {
		pe.Endian = binary.BigEndian
		slog.Debug("section in packet capture stream is big endian")
	} else {
		pe.Endian = binary.LittleEndian
		slog.Debug("section in packet capture stream is little endian")
	}
	pe.shbLen = pe.Endian.Uint32(pe.shb[4:8])
	if pe.shbLen < minSHBLen || pe.shbLen > maxSHBLen || pe.shbLen%4 != 0 {
		slog.Error("invalid section header block length",
			slog.Uint64("length", uint64(pe.shbLen)))
		return false
	}
	return true
}

// Option represents a pcapng option, consisting of a Code uniquely identifying
// the type of option, as well as its (binary) value in form of an octet string.
type Option struct {
	Code  uint16 // Option Code
	Value []byte // Value
}

const (
	// OptEndofOpt signals the end of options.
	OptEndofOpt = uint16(0)
	// OptComment contains a comment in form of an UTF-8 string.
	OptComment = uint16(1)
	// OptSHBHardware contains the description of the hardware used to create this
	// section, in form of an UTF-8 string.
	OptSHBHardware = uint16(2)
	// OptSHBOS contains the name of the operating system used to create this
	// section, in form of an UTF-8 string.
	OptSHBOS = uint16(3)
	// OptSHBUserAppl contains the name of the application used to create this
	// section, in form of an UTF-8 string.
	OptSHBUserAppl = uint16(4)
)

// NewOption returns a new pcapng Option read from the buffer using the
// given endianness, as well as the number of octets to skip over to arrive
// at the next option. If the last option is reached, then nil is returned,
// together with the amount of octets to skip past the end-of-options mark. If
// buff is too short to hold a complete option, nil and a zero skip are
// returned.
func NewOption(buff []byte, endian binary.ByteOrder) (opt *Option, skip uint) {
	if len(buff) < 4 {
		return nil, 0
	}
	code := endian.Uint16(buff)
	length := endian.Uint16(buff[2:4])
	if 4+int(length) > len(buff) {
		return nil, 0
	}
	// Calculate overall length of this option, and make sure to align it to
	// the next 32bit boundary.
	skip = uint(2+2) + uint(length)
	if skip&0x3 != 0 {
		skip += 4 - (skip & 0x3)
	}
	// If it's not the end-of-options marker, then return a PcapngOption
	// object, otherwise simply return nil. The amount of octets to skip is
	// already calculated correctly for all cases.
	if code != OptEndofOpt || length != 0 {
		opt = &Option{Code: code, Value: buff[4 : 4+int(length)]}
	}
	return
}

// String returns an option's value as a string instead of octets, assuming
// UTF-8 encoding.
func (o *Option) String() string {
	return string(o.Value)
}

// Bytes returns the octets encoding the option, using the specified
// endianness. Values longer than an option can encode get truncated.
func (o *Option) Bytes(endian binary.ByteOrder) (b []byte) {
	if o == nil {
		return []byte{0, 0, 0, 0}
	}
	value := o.Value
	if len(value) > maxOptionValueLen {
		value = value[:maxOptionValueLen]
	}
	length := len(value)
	by := make([]byte, 4+length, 4+length+3)
	endian.PutUint16(by[0:2], o.Code)
	endian.PutUint16(by[2:4], uint16(length))
	copy(by[4:], value)
	if length&0x3 != 0 {
		by = append(by, make([]byte, 4-(length&0x3))...)
	}
	return by
}

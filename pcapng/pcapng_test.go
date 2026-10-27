// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

package pcapng

import (
	"bytes"
	"encoding/binary"
	"io"
	"log/slog"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// sinkFunc adapts a plain function to an io.Writer, so tests can inject sink
// behavior, such as failing or short writes.
type sinkFunc func([]byte) (int, error)

func (f sinkFunc) Write(b []byte) (int, error) { return f(b) }

var _ = Describe("pcapng", func() {

	BeforeEach(func() {
		DeferCleanup(slog.SetDefault, slog.Default())
		slog.SetDefault(slog.New(slog.NewTextHandler(GinkgoWriter, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})))
	})

	It("Encodes opts", func() {
		bbig := (&Option{Code: uint16(42), Value: []byte("Go")}).
			Bytes(binary.BigEndian)
		Expect(bbig).Should(HaveLen(2 + 2 + 4))
		Expect(bbig).Should(Equal([]byte{0, 42, 0, 2, byte('G'), byte('o'), 0, 0}))

		blittle := (&Option{Code: uint16(42), Value: []byte("Go")}).
			Bytes(binary.LittleEndian)
		Expect(blittle).Should(HaveLen(2 + 2 + 4))
		Expect(blittle).Should(Equal([]byte{42, 0, 2, 0, byte('G'), byte('o'), 0, 0}))
	})

	It("Encodes end-of-opts", func() {
		b := (&Option{}).Bytes(binary.BigEndian)
		Expect(b).Should(HaveLen(4))
		Expect(b).Should(Equal([]byte{0, 0, 0, 0}))
	})

	It("Decodes opts", func() {
		bbig := (&Option{Code: OptComment, Value: []byte("Kuhbernetes")}).
			Bytes(binary.BigEndian)
		opt, skip := NewOption(bbig, binary.BigEndian)
		Expect(opt.Code).Should(Equal(OptComment))
		Expect(opt.String()).Should(Equal("Kuhbernetes"))
		Expect(skip).Should(Equal(uint(16)))
	})

	It("Decodes end-of-opts", func() {
		opt, skip := NewOption([]byte{0, 0, 0, 0}, binary.BigEndian)
		Expect(opt).Should(BeNil())
		Expect(skip).Should(Equal(uint(4)))
	})

	DescribeTable("rejects truncated opts",
		func(b []byte) {
			opt, skip := NewOption(b, binary.BigEndian)
			Expect(opt).To(BeNil())
			Expect(skip).To(BeZero())
		},
		Entry("empty", []byte{}),
		Entry("short header", []byte{0, 1, 0}),
		Entry("short value", []byte{0, 1, 0, 8, 'a', 'b'}),
	)

	It("truncates overly long option values when encoding", func() {
		b := (&Option{Code: OptComment, Value: make([]byte, maxOptionValueLen+42)}).
			Bytes(binary.BigEndian)
		Expect(b).To(HaveLen(4 + maxOptionValueLen + 1))
		Expect(b[2:4]).To(Equal([]byte{0xff, 0xff}))
	})

	DescribeTable("passes through streams with invalid SHB lengths",
		func(length []byte) {
			var b bytes.Buffer
			se := NewStreamEditor(&b, nil, "", false)
			in := append([]byte{0x0a, 0x0d, 0x0d, 0x0a}, length...)
			in = append(in, 0x1a, 0x2b, 0x3c, 0x4d, 0x00, 0x01, 0x00, 0x00)
			Expect(se.Write(in)).To(Equal(len(in)))
			Expect(b.Bytes()).To(Equal(in))
		},
		Entry("zero", []byte{0, 0, 0, 0}),
		Entry("too short", []byte{0, 0, 0, 0x0c}),
		Entry("misaligned", []byte{0, 0, 0, 0x1d}),
		Entry("too long", []byte{0xff, 0xff, 0xff, 0xfc}),
	)

	DescribeTable("reports sink failures without claiming octets as written",
		func(sink sinkFunc, expectedErr error) {
			calls := 0
			se := NewStreamEditor(sinkFunc(func(b []byte) (int, error) {
				calls++
				return sink(b)
			}), nil, "", false)
			in := []byte{0x0a, 0x0d, 0x0d, 0x0a, 0, 0, 0, 0,
				0x1a, 0x2b, 0x3c, 0x4d, 0x00, 0x01, 0x00, 0x00}
			n, err := se.Write(in)
			Expect(err).To(MatchError(expectedErr))
			Expect(n).To(BeZero())
			n, err = se.Write(in)
			Expect(err).To(MatchError(expectedErr))
			Expect(n).To(BeZero())
			Expect(calls).To(Equal(1))
		},
		Entry("failing sink",
			sinkFunc(func([]byte) (int, error) { return 1, io.ErrClosedPipe }), io.ErrClosedPipe),
		Entry("short-writing sink",
			sinkFunc(func([]byte) (int, error) { return 1, nil }), io.ErrShortWrite),
	)

	It("stops at an option overrunning the SHB", func() {
		var b bytes.Buffer
		se := NewStreamEditor(&b, nil, "", false)
		Expect(se.Write([]byte{
			0x0a, 0x0d, 0x0d, 0x0a,
			0x00, 0x00, 0x00, 0x20,
			0x1a, 0x2b, 0x3c, 0x4d,
			0x00, 0x01, 0x00, 0x00,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0x00, 0x02, 0xff, 0xff, // option claiming 64k of value
			0x00, 0x00, 0x00, 0x20,
		})).Error().NotTo(HaveOccurred())
		Expect(b.Bytes()[4:8]).To(Equal([]byte{0x00, 0x00, 0x00, 0x78}))
	})

	It("drops an existing comment that would overflow with target information", func() {
		const commentLen = 65532
		shbLen := minSHBLen + 4 + commentLen
		in := []byte{
			0x0a, 0x0d, 0x0d, 0x0a,
			byte(shbLen >> 24), byte(shbLen >> 16), byte(shbLen >> 8), byte(shbLen),
			0x1a, 0x2b, 0x3c, 0x4d,
			0x00, 0x01, 0x00, 0x00,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0x00, 0x01, byte(commentLen >> 8), byte(commentLen & 0xff),
		}
		in = append(in, bytes.Repeat([]byte{'A'}, commentLen)...)
		in = append(in, in[4:8]...)

		var b bytes.Buffer
		se := NewStreamEditor(&b, nil, "", false)
		Expect(se.Write(in)).To(Equal(len(in)))
		Expect(b.Bytes()[4:8]).To(Equal([]byte{0x00, 0x00, 0x00, 0x78}))
		Expect(b.Bytes()).NotTo(ContainElement(byte('A')))
	})

	It("Edits SHB creating new comment", func() {
		var b bytes.Buffer
		se := NewStreamEditor(&b, nil, "", false)
		Expect(se).ShouldNot(BeNil())
		n, err := se.Write([]byte{
			0x0a, 0x0d, 0x0d, 0x0a, // SHB block type
			0x00, 0x00, 0x00, 0x1c, // total block length
			0x1a, 0x2b, 0x3c, 0x4d, // byte-order magic
			0x00, 0x01, 0x00, 0x00, // major, minor
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, // section length unknown
			0x00, 0x00, 0x00, 0x1c, // total block length
			0x01, 0x02, 0x03, 0x04, 0x05, // test overspill
		})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(n).ShouldNot(BeZero())
		Expect(b.Bytes()).Should(Equal([]byte{
			0x0a, 0x0d, 0x0d, 0x0a,
			0x00, 0x00, 0x00, 0x78,
			0x1a, 0x2b, 0x3c, 0x4d,
			0x00, 0x01, 0x00, 0x00,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,

			0x00, 0x01, 0x00, 85,
			45, 45, 45, 10, 35, 32, 99, 97, 112, 116, 117, 114, 101, 32, 116, 97, 114, 103, 101, 116, 32, 105, 110, 102, 111, 114, 109, 97, 116, 105, 111, 110, 10, 99, 111, 110, 116, 97, 105, 110, 101, 114, 45, 110, 97, 109, 101, 58, 32, 34, 34, 10, 99, 111, 110, 116, 97, 105, 110, 101, 114, 45, 116, 121, 112, 101, 58, 32, 34, 34, 10, 110, 111, 100, 101, 45, 110, 97, 109, 101, 58, 32, 34, 34, 10, 0, 0, 0,

			0x00, 0x00, 0x00, 0x78,
			0x01, 0x02, 0x03, 0x04, 0x05,
		}))
	})

	It("Edits SHB editing existing comment", func() {
		var b bytes.Buffer
		se := NewStreamEditor(&b, nil, "", false)
		Expect(se).ShouldNot(BeNil())
		n, err := se.Write([]byte{
			0x0a, 0x0d, 0x0d, 0x0a, // SHB block type
			0x00, 0x00, 0x00, 0x24, // total block length
			0x1a, 0x2b, 0x3c, 0x4d, // byte-order magic
			0x00, 0x01, 0x00, 0x00, // major, minor
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, // section length unknown
			0x00, 0x01, 0x00, 0x03, // comment option
			0x41, 0x42, 0x43, 0x00, // "ABC"
			0x00, 0x00, 0x00, 0x24, // total block length
			0x01, 0x02, 0x03, 0x04, 0x05, // test overspill
		})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(n).ShouldNot(BeZero())
		Expect(b.Bytes()).Should(Equal([]byte{
			0x0a, 0x0d, 0x0d, 0x0a,
			0x00, 0x00, 0x00, 0x7c,
			0x1a, 0x2b, 0x3c, 0x4d,
			0x00, 0x01, 0x00, 0x00,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,

			0x00, 0x01, 0x00, 89,
			65, 66, 67, 10,
			45, 45, 45, 10, 35, 32, 99, 97, 112, 116, 117, 114, 101, 32, 116, 97, 114, 103, 101, 116, 32, 105, 110, 102, 111, 114, 109, 97, 116, 105, 111, 110, 10, 99, 111, 110, 116, 97, 105, 110, 101, 114, 45, 110, 97, 109, 101, 58, 32, 34, 34, 10, 99, 111, 110, 116, 97, 105, 110, 101, 114, 45, 116, 121, 112, 101, 58, 32, 34, 34, 10, 110, 111, 100, 101, 45, 110, 97, 109, 101, 58, 32, 34, 34, 10, 0, 0, 0,

			0x00, 0x00, 0x00, 0x7c,
			0x01, 0x02, 0x03, 0x04, 0x05,
		}))
	})

	It("Edits SHB editing existing comment, replacing target data", func() {
		var b bytes.Buffer
		se := NewStreamEditor(&b, nil, "", false)
		Expect(se).ShouldNot(BeNil())
		n, err := se.Write([]byte{
			0x0a, 0x0d, 0x0d, 0x0a, // SHB block type
			0x00, 0x00, 0x00, 0x48, // total block length
			0x1a, 0x2b, 0x3c, 0x4d, // byte-order magic
			0x00, 0x01, 0x00, 0x00, // major, minor
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, // section length unknown
			0x00, 0x01, 0x00, 0x25, // comment option
			0x41, 0x42, 0x43, 0x0a, // "ABC\n"
			0x2d, 0x2d, 0x2d, 0x0a, // # capture target information\n...
			0x23, 0x20, 0x63, 0x61,
			0x70, 0x74, 0x75, 0x72,
			0x65, 0x20, 0x74, 0x61,
			0x72, 0x67, 0x65, 0x74,
			0x20, 0x69, 0x6e, 0x66,
			0x6f, 0x72, 0x6d, 0x61,
			0x74, 0x69, 0x6f, 0x6e,
			0x0a, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x48, // total block length
			0x01, 0x02, 0x03, 0x04, 0x05, // test overspill
		})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(n).ShouldNot(BeZero())
		Expect(b.Bytes()).Should(Equal([]byte{
			0x0a, 0x0d, 0x0d, 0x0a,
			0x00, 0x00, 0x00, 0x7c,
			0x1a, 0x2b, 0x3c, 0x4d,
			0x00, 0x01, 0x00, 0x00,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,

			0x00, 0x01, 0x00, 89,
			65, 66, 67, 10,
			45, 45, 45, 10, 35, 32, 99, 97, 112, 116, 117, 114, 101, 32, 116, 97, 114, 103, 101, 116, 32, 105, 110, 102, 111, 114, 109, 97, 116, 105, 111, 110, 10, 99, 111, 110, 116, 97, 105, 110, 101, 114, 45, 110, 97, 109, 101, 58, 32, 34, 34, 10, 99, 111, 110, 116, 97, 105, 110, 101, 114, 45, 116, 121, 112, 101, 58, 32, 34, 34, 10, 110, 111, 100, 101, 45, 110, 97, 109, 101, 58, 32, 34, 34, 10, 0, 0, 0,

			0x00, 0x00, 0x00, 0x7c,
			0x01, 0x02, 0x03, 0x04, 0x05,
		}))
	})

})

// (c) Siemens AG 2023
//
// SPDX-License-Identifier: MIT

package pcapng

import (
	"bytes"
	"encoding/binary"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("pcapng", func() {

	It("Encodes opts", func() {
		bbig := (&Option{Code: uint16(42), Value: []byte("Go")}).
			Bytes(binary.BigEndian)
		Expect(len(bbig)).Should(Equal(2 + 2 + 4))
		Expect(bbig).Should(Equal([]byte{0, 42, 0, 2, byte('G'), byte('o'), 0, 0}))

		blittle := (&Option{Code: uint16(42), Value: []byte("Go")}).
			Bytes(binary.LittleEndian)
		Expect(len(blittle)).Should(Equal(2 + 2 + 4))
		Expect(blittle).Should(Equal([]byte{42, 0, 2, 0, byte('G'), byte('o'), 0, 0}))
	})

	It("Encodes end-of-opts", func() {
		b := (&Option{}).Bytes(binary.BigEndian)
		Expect(len(b)).Should(Equal(4))
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

package media

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/sunrise2575/AutoAVS/filesys"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"
)

func convertUTF16LEtoUTF8(in []byte) []byte {
	ret := &bytes.Buffer{}
	b := make([]byte, 4)
	for i := 0; i < len(in); i += 2 {
		u16 := (uint16(in[i+1]) << 8) + uint16(in[i])
		r := utf16.Decode([]uint16{u16})
		n := utf8.EncodeRune(b, r[0])
		ret.Write(b[:n])
	}
	return ret.Bytes()
}

func convertUTF16BEtoUTF8(in []byte) []byte {
	ret := &bytes.Buffer{}
	b := make([]byte, 4)
	for i := 0; i < len(in); i += 2 {
		u16 := (uint16(in[i]) << 8) + uint16(in[i+1])
		r := utf16.Decode([]uint16{u16})
		n := utf8.EncodeRune(b, r[0])
		ret.Write(b[:n])
	}
	return ret.Bytes()
}

func convertCP949toUTF8(in []byte) []byte {
	reader := transform.NewReader(bytes.NewReader(in), korean.EUCKR.NewDecoder())
	d, _ := ioutil.ReadAll(reader)
	return d
}

func haveBrokenChar(in []byte) bool {
	for len(in) > 0 {
		r, size := utf8.DecodeRune(in)
		if r == '�' {
			return true
		}
		in = in[size:]
	}

	return false
}

// TextToUTF8 ...
func TextToUTF8(oldFilePath, newFilePath string) error {
	if !filesys.IsFile(oldFilePath) {
		return fmt.Errorf("not a file")
	}

	data, e := ioutil.ReadFile(oldFilePath)
	if e != nil {
		panic(e)
	}

	// convert encoding to utf8
	var u8 []byte
	switch {
	case data[0] == 0x00 && data[1] == 0x00 && data[2] == 0xFE && data[3] == 0xFF:
		return fmt.Errorf("%s is UTF-32 BE; not supported type", oldFilePath)
	case data[0] == 0xFF && data[1] == 0xFE && data[2] == 0x00 && data[3] == 0x00:
		return fmt.Errorf("%s is UTF-32 LE; not supported type", oldFilePath)
	case data[0] == 0xFE && data[1] == 0xFF:
		log.Printf("%s is UTF-16 BE\n", oldFilePath)
		u8 = convertUTF16BEtoUTF8(data[2:])
	case data[0] == 0xFF && data[1] == 0xFE:
		log.Printf("%s is UTF-16 LE\n", oldFilePath)
		u8 = convertUTF16LEtoUTF8(data[2:])
	default:
		if utf8.Valid(data) {
			if data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
				log.Printf("%s is UTF-8 (BOM)\n", oldFilePath)
				u8 = data[3:]
			} else {
				log.Printf("%s is already UTF-8, skip conversion\n", oldFilePath)
				return nil
			}
		} else {
			log.Printf("%s is CP949\n", oldFilePath)
			u8 = convertCP949toUTF8(data)
		}
	}

	if haveBrokenChar(u8) {
		return fmt.Errorf("conversion result of %s will contains broken UTF-8 character, abort conversion and do nothing to your file", oldFilePath)
	}

	if e = ioutil.WriteFile(newFilePath, u8, 0644); e != nil {
		return e
	}

	return nil
}

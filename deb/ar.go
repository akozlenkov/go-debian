/* {{{ Copyright (c) Paul R. Tagliamonte <paultag@debian.org>, 2015
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE. }}} */

package deb // import "github.com/akozlenkov/go-debian/deb"

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ArEntry {{{

// Container type to access the different parts of a Debian `ar(1)` Archive.
//
// The most interesting parts of this are the `Name` attribute, Data
// `io.Reader`, and the Tarfile helpers. This will allow the developer to
// programmatically inspect the information inside without forcing her to
// unpack the .deb to the filesystem.
type ArEntry struct {
	Name      string
	Timestamp int64
	OwnerID   int64
	GroupID   int64
	FileMode  string
	Size      int64
	Data      *io.SectionReader
}

// }}}

// Ar {{{

// This struct encapsulates a Debian .deb flavored `ar(1)` archive.
type Ar struct {
	in     io.ReaderAt
	offset int64
}

// LoadAr {{{

// Load an Ar archive reader from an io.ReaderAt
func LoadAr(in io.ReaderAt) (*Ar, error) {
	offset, err := checkAr(in)
	if err != nil {
		return nil, err
	}
	debFile := Ar{in: in, offset: offset}
	return &debFile, nil
}

// }}}

// Next {{{

// Function to jump to the next file in the Debian `ar(1)` archive, and
// return the next member.
func (d *Ar) Next() (*ArEntry, error) {
	line := make([]byte, 60)

	count, err := d.in.ReadAt(line, d.offset)
	if err != nil {
		return nil, err
	}
	if count == 1 && line[0] == '\n' {
		return nil, io.EOF
	}
	if count != 60 {
		return nil, fmt.Errorf("Caught a short read at the end")
	}
	entry, err := parseArEntry(line)
	if err != nil {
		return nil, err
	}

	entry.Data = io.NewSectionReader(d.in, d.offset+int64(count), entry.Size)
	d.offset += int64(count) + entry.Size + (entry.Size % 2)

	return entry, nil
}

// }}}

// toDecimal {{{

// Take a byte array, and return an int64
func toDecimal(input string) (int64, error) {
	out, err := strconv.Atoi(input)
	return int64(out), err
}

// }}}

// }}}

// AR Format Hackery {{{

// parseArEntry {{{

// Take the AR format line, and create an ArEntry (without .Data set)
// to be returned to the user later.
//
// +-------------------------------------------------------
// | Offset  Length  Name                         Format
// +-------------------------------------------------------
// | 0       16      File name                    ASCII
// | 16      12      File modification timestamp  Decimal
// | 28      6       Owner ID                     Decimal
// | 34      6       Group ID                     Decimal
// | 40      8       File mode                    Octal
// | 48      10      File size in bytes           Decimal
// | 58      2       File magic                   0x60 0x0A
type entryField struct {
	Name    string
	Pointer *int64
}

func parseArEntry(line []byte) (*ArEntry, error) {
	if len(line) != 60 {
		return nil, fmt.Errorf("Malformed file entry line length")
	}

	if line[58] != 0x60 && line[59] != 0x0A {
		return nil, fmt.Errorf("Malformed file entry line endings")
	}

	entry := ArEntry{
		Name:     strings.TrimSuffix(strings.TrimSpace(string(line[0:16])), "/"),
		FileMode: strings.TrimSpace(string(line[40:48])),
	}

	for target, value := range map[entryField][]byte{
		entryField{"Timestamp", &entry.Timestamp}: line[16:28],
		entryField{"OwnerID", &entry.OwnerID}:     line[28:34],
		entryField{"GroupID", &entry.GroupID}:     line[34:40],
		entryField{"Size", &entry.Size}:           line[48:58],
	} {
		input := strings.TrimSpace(string(value))
		if input == "" {
			continue
		}

		intValue, err := toDecimal(input)
		if err != nil {
			return nil, fmt.Errorf("failed to parse entry %s: %w", target.Name, err)
		}
		*target.Pointer = intValue
	}

	return &entry, nil
}

// }}}

// checkAr {{{

// Given a brand spank'n new os.File entry, go ahead and make sure it looks
// like an `ar(1)` archive, and not some random file.
func checkAr(reader io.ReaderAt) (int64, error) {
	header := make([]byte, 8)
	if _, err := reader.ReadAt(header, 0); err != nil {
		return 0, err
	}
	if string(header) != "!<arch>\n" {
		return 0, fmt.Errorf("Header doesn't look right!")
	}
	return int64(len(header)), nil
}

// }}}

// }}}

// vim: foldmethod=marker

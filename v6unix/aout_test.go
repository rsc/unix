// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package v6unix

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
)

func TestParseAout(t *testing.T) {
	ar, err := txtar.ParseFile("disk.txtar")
	if err != nil {
		t.Error(err)
	}
	for _, f := range ar.Files {
		if !strings.HasPrefix(f.Name, "/etc/init") {
			continue
		}
		t.Log(f.Name)
		dec, err := base64.StdEncoding.DecodeString(string(f.Data))
		if err != nil {
			t.Fatal(err)
		}
		of, err := ParseAout(bytes.NewBuffer(dec))
		if err != Errno(0) || of.hdr.MagicNum != 0o410 {
			t.Fatal(err)
		}
	}
}

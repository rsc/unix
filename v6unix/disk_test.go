// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package unix

import (
	"bytes"
	"strings"
	"testing"
)

var disktab = []struct {
	inum  uint16
	mode  uint16
	major uint8
	minor uint8
	name  string
}{
	{1, _IFDIR | 0o555, 0, 0, "/"},
	{1, _IFDIR | 0o555, 0, 0, "/."},
	{1, _IFDIR | 0o555, 0, 0, "/.."},
	{2, _IFDIR | 0o555, 0, 0, "/dev"},
	{2, _IFDIR | 0o555, 0, 0, "/dev/."},
	{1, _IFDIR | 0o555, 0, 0, "/dev/.."},
	{3, _IFCHR | 0o600, 1, 1, "/dev/null"},
	{4, _IFCHR | 0o600, 2, 1, "/dev/mem"},
	{5, _IFCHR | 0o600, 2, 2, "/dev/kmem"},
	{6, _IFCHR | 0o600, 3, 0, "/dev/tty"},
	{7, _IFCHR | 0o600, 3, 1, "/dev/tty1"},
	{8, _IFCHR | 0o600, 3, 2, "/dev/tty2"},
	{9, _IFCHR | 0o600, 3, 3, "/dev/tty3"},
	{10, _IFCHR | 0o600, 3, 4, "/dev/tty4"},
	{11, _IFCHR | 0o600, 3, 5, "/dev/tty5"},
	{12, _IFCHR | 0o600, 3, 6, "/dev/tty6"},
	{13, _IFCHR | 0o600, 3, 7, "/dev/tty7"},
	{14, _IFCHR | 0o600, 3, 8, "/dev/tty8"},
}

func TestNewDisk(t *testing.T) {
	d, err := newDisk(FS)
	if err != nil {
		t.Fatal(err)
	}
	var sys System
	sys.Disk = d
	p := sys.newProc()

	for _, tab := range disktab {
		p.Error = 0
		var st stat
		p.stat(tab.name, &st)
		if p.Error != 0 {
			t.Errorf("stat %s: %v", tab.name, p.Error)
			continue
		}
		if st.inum != tab.inum || st.mode != _IALLOC|tab.mode || st.major != tab.major || st.minor != tab.minor {
			t.Errorf("stat %s: have #%d %06o %d,%d, want #%d %06o %d,%d", tab.name, st.inum, st.mode, st.major, st.minor, tab.inum, tab.mode|_IALLOC, tab.major, tab.minor)
		}
	}

	for _, tab := range disktab {
		if !strings.HasSuffix(tab.name, ".") && int(tab.inum) < len(d.inodes) {
			ip := d.inodes[tab.inum]
			if ip.count != 0 {
				t.Errorf("inode #%d %s count = %d, want %d", tab.inum, tab.name, ip.count, 0)
			}
		}
	}
}

func TestUnlink(t *testing.T) {
	d, err := newDisk(FS)
	if err != nil {
		t.Fatal(err)
	}
	var sys System
	sys.Disk = d
	p := sys.newProc()

	p.unlink("/dev/null")
	if p.Error != 0 {
		t.Fatalf("unlink: %v", p.Error)
	}

	var st stat
	p.stat("/dev/null", &st)
	if p.Error == 0 {
		t.Fatalf("stat /dev/null succeeded after unlink")
	}
}

func TestEcho(t *testing.T) {
	sys, err := NewSystem(FS)
	if err != nil {
		t.Fatal(err)
	}
	aout, err := sys.ReadFile("/bin/echo")
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	_, err = sys.Start(aout, []string{"echo", "hello", "world"}, &stdout)
	if err != nil {
		t.Fatal(err)
	}
	sys.Wait()
	if stdout.String() != "hello world\n" || stderr.String() != "" {
		t.Errorf("have stdout=%q stderr=%q\nwant stdin=%q stdout=%q stderr=%q\n", stdout.String(), stderr.String(), "", "hello world", "")
	}
}

func TestLs(t *testing.T) {
	t.Skip("ls")

	sys, err := NewSystem(FS)
	if err != nil {
		t.Fatal(err)
	}
	aout, err := sys.ReadFile("/bin/ls")
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	_, err = sys.Start(aout, []string{"ls", "-l", "/dev"}, &stdout)
	if err != nil {
		t.Fatal(err)
	}
	sys.Wait()
	if stdout.String() != "hello world\n" || stderr.String() != "" {
		t.Errorf("have stdout=%q stderr=%q\nwant stdin=%q stdout=%q stderr=%q\n", stdout.String(), stderr.String(), "", "hello world", "")
	}
}

func TestDate(t *testing.T) {
	sys, err := NewSystem(FS)
	if err != nil {
		t.Fatal(err)
	}
	aout, err := sys.ReadFile("/bin/date")
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	_, err = sys.Start(aout, []string{"date"}, &stdout)
	if err != nil {
		t.Fatal(err)
	}
	sys.Wait()
	const want = "Thu Jan  1 19:00:00 EST 1970\n"
	if stdout.String() != want || stderr.String() != "" {
		t.Errorf("have stdout=%q stderr=%q\nwant stdin=%q stdout=%q stderr=%q\n", stdout.String(), stderr.String(), "", want, "")
	}
}

func TestShellDate(t *testing.T) {
	sys, err := NewSystem(FS)
	if err != nil {
		t.Fatal(err)
	}
	aout, err := sys.ReadFile("/bin/sh")
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range []byte("date\n") {
		sys.TTY[8].WriteByte(c)
	}
	var stdout, stderr bytes.Buffer
	_, err = sys.Start(aout, []string{"sh"}, &stdout)
	if err != nil {
		t.Fatal(err)
	}
	sys.Wait()
	want := "# Thu Aug 14 22:04:50 EDT 1975\n# "
	if stdout.String() != want || stderr.String() != "" {
		t.Fatalf("have stdout=%q stderr=%q\nwant stdin=%q stdout=%q stderr=%q\n", stdout.String(), stderr.String(), "", want, "")
	}

	if sys.TTYRead != 1<<8 {
		t.Fatalf("have TTYRead=%06o, want %06o", sys.TTYRead, 1<<8)
	}
	for _, c := range []byte("echo hello world\n") {
		sys.TTY[8].WriteByte(c)
	}
	stdout.Reset()
	sys.Wait()
	want = "hello world\n# "
	if stdout.String() != want || stderr.String() != "" {
		t.Fatalf("have stdout=%q stderr=%q\nwant stdin=%q stdout=%q stderr=%q\n", stdout.String(), stderr.String(), "", want, "")
	}
}

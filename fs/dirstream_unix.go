//go:build !darwin

// Copyright 2019 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import (
	"bytes"
	"sync"
	"syscall"
	"unsafe"

	"github.com/hanwen/go-fuse/v2/fuse"
	"golang.org/x/sys/unix"
)

type loopbackDirStream struct {
	buf []byte

	// Protects mutable members
	mu sync.Mutex

	// mutable
	todo      []byte
	todoErrno syscall.Errno
	fd        int
}

// NewLoopbackDirStream open a directory for reading as a DirStream
func NewLoopbackDirStream(name string) (DirStream, syscall.Errno) {
	// TODO: should return concrete type.
	fd, err := syscall.Open(name, syscall.O_DIRECTORY, 0755)
	if err != nil {
		return nil, ToErrno(err)
	}

	ds := &loopbackDirStream{
		buf: make([]byte, 4096),
		fd:  fd,
	}

	ds.load()
	return ds, OK
}

func (ds *loopbackDirStream) Close() {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	if ds.fd != -1 {
		syscall.Close(ds.fd)
		ds.fd = -1
	}
}

func (ds *loopbackDirStream) HasNext() bool {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	return len(ds.todo) > 0 || ds.todoErrno != 0
}

func (ds *loopbackDirStream) Next() (fuse.DirEntry, syscall.Errno) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if ds.todoErrno != 0 {
		return fuse.DirEntry{}, ds.todoErrno
	}

	// We can't use syscall.Dirent here, because it declares a
	// [256]byte name, which may run beyond the end of ds.todo.
	// when that happens in the race detector, it causes a panic
	// "converted pointer straddles multiple allocations"
	de := (*dirent)(unsafe.Pointer(&ds.todo[0]))

	nameBytes := ds.todo[unsafe.Offsetof(dirent{}.Name):de.Reclen]
	ds.todo = ds.todo[de.Reclen:]

	l := bytes.IndexByte(nameBytes, 0)
	if l >= 0 {
		nameBytes = nameBytes[:l]
	}
	result := fuse.DirEntry{
		Ino:  de.Ino,
		Mode: (uint32(de.Type) << 12),
		Name: string(nameBytes),
		Off:  uint64(de.Off),
	}
	ds.load()
	return result, 0
}

func (ds *loopbackDirStream) load() {
	if len(ds.todo) > 0 {
		return
	}

	n, err := unix.Getdents(ds.fd, ds.buf)
	if n < 0 {
		n = 0
	}
	ds.todo = ds.buf[:n]
	ds.todoErrno = ToErrno(err)
}

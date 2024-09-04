package fs

// Like syscall.Dirent, but without the [256]byte name.
type dirent struct {
	Ino    uint64
	Off    int64
	Reclen uint16
	Type   uint8
	Name   [1]uint8 // align to 4 bytes for 32 bits.
}

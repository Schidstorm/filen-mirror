package mirror

import (
	"strconv"
	"unsafe"

	"github.com/rs/zerolog/log"
	"golang.org/x/sys/unix"
)

func fastReadDirDirs(path string, cb func(p string, isDir bool, continueDescending *bool)) error {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY, 0)
	if err != nil {
		return err
	}
	defer unix.Close(fd)

	buf := make([]byte, 1024)

	for {
		n, err := unix.Getdents(fd, buf)
		if err != nil {
			return err
		}
		if n == 0 {
			break
		}

		b := buf[:n]
		for len(b) > 0 {
			dirent := (*unix.Dirent)(unsafe.Pointer(&b[0]))
			if dirent.Ino != 0 {
				name := unix.ByteSliceToString(int8SliceToByteSlice(dirent.Name[:]))
				var continueDescending bool = true
				if dirent.Type == unix.DT_DIR {
					if name != "." && name != ".." {
						cb(path+"/"+name, true, &continueDescending)
						if continueDescending {
							err := fastReadDirDirs(path+"/"+name, cb)
							if err != nil {
								log.Warn().Err(err).Msgf("Failed to read directory: %s", path+"/"+name)
								continue
							}
						}
					}
				} else {
					cb(path+"/"+name, false, &continueDescending)
				}
			}
			b = b[dirent.Reclen:]
		}
	}

	return nil
}

func int8SliceToByteSlice(a []int8) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(&a[0])), len(a))
}

func maybeString(m map[string]any, key string) int64 {
	switch v := m[key].(type) {
	case string:
		i, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0
		}
		return i
	case float64:
		return int64(v)
	}
	return 0
}

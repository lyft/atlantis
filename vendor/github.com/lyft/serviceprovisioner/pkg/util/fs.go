package util

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
)

// Use a large 1M buffer in case we are on an NFS filesystem
const DefaultBufsize = 1024 * 1024

func HashFileSize(ctx context.Context, name string, bufsize int) (string, error) {
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var buf []byte
	if bufsize > 0 {
		buf = make([]byte, bufsize)
	}

	h := md5.New()

	if _, err := CopyBuffer(ctx, h, f, buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// HashFile returns the MD5 hash of file name.  The hash operation is aborted
// if the context is canceled.
func HashFile(ctx context.Context, name string) (string, error) {
	return HashFileSize(ctx, name, -1)
}

// Copy is essentially the same as io.Copy, but will stop if the Context
// is cancelled.  Additionally, a larger buffer 1M is used.
func Copy(ctx context.Context, dst io.Writer, src io.Reader) (written int64, err error) {
	return CopyBuffer(ctx, dst, src, nil)
}

func CopyBuffer(ctx context.Context, dst io.Writer, src io.Reader, buf []byte) (written int64, err error) {
	// Don't support WriterTo and ReaderFrom since we can't cancel them
	if buf == nil {
		size := DefaultBufsize
		if l, ok := src.(*io.LimitedReader); ok && int64(size) > l.N {
			if l.N < 1 {
				size = 1
			} else {
				size = int(l.N)
			}
		}
		buf = make([]byte, size)
	}
	ch := ctx.Done()
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
		select {
		case <-ch:
			if err = ctx.Err(); err == nil {
				err = context.Canceled
			}
			return written, err
		default:
			// Ok
		}
	}
	return written, err
}

// FileExists returns if file name exists and is a regular file.
func FileExists(name string) bool {
	fi, err := os.Stat(name)
	return err == nil && fi.Mode().IsRegular()
}

// ContextCanceled returns if Context ctx is canceled.
func ContextCanceled(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

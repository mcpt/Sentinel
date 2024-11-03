package compression

import (
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"os"
)

type Compressor interface {
	Compress(src []string, dst string) error
}

type ZlibCompressor struct {
	level int // 1-9 see https://pkg.go.dev/compress/flate#pkg-constants
}
type GzipCompressor struct{}

func NewCompressor(format string, level int) (Compressor, error) {
	switch format {
	case "zlib":
		return &ZlibCompressor{level: level}, nil
	case "gzip":
		return &GzipCompressor{}, nil
	default:
		return nil, fmt.Errorf("unsupported compression format: %s", format)
	}
}

func (c *ZlibCompressor) Compress(src []string, dst string) error {
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()

	zw, err := zlib.NewWriterLevel(f, c.level)
	if err != nil {
		return err
	}
	defer zw.Close()

	for _, file := range src {
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(zw, f)
		if err != nil {
			return err
		}
	}
	err = zw.Flush()
	if err != nil {
		return err
	}
	return nil
}

func (c *GzipCompressor) Compress(src []string, dst string) error {
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	for _, file := range src {
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(gw, f)
		if err != nil {
			return err
		}
	}
	return nil
}

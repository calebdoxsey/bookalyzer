// Code generated by "esc -o files_gen.go -pkg main tpl assets"; DO NOT EDIT.

package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"sync"
	"time"
)

type _escLocalFS struct{}

var _escLocal _escLocalFS

type _escStaticFS struct{}

var _escStatic _escStaticFS

type _escDirectory struct {
	fs   http.FileSystem
	name string
}

type _escFile struct {
	compressed string
	size       int64
	modtime    int64
	local      string
	isDir      bool

	once sync.Once
	data []byte
	name string
}

func (_escLocalFS) Open(name string) (http.File, error) {
	f, present := _escData[path.Clean(name)]
	if !present {
		return nil, os.ErrNotExist
	}
	return os.Open(f.local)
}

func (_escStaticFS) prepare(name string) (*_escFile, error) {
	f, present := _escData[path.Clean(name)]
	if !present {
		return nil, os.ErrNotExist
	}
	var err error
	f.once.Do(func() {
		f.name = path.Base(name)
		if f.size == 0 {
			return
		}
		var gr *gzip.Reader
		b64 := base64.NewDecoder(base64.StdEncoding, bytes.NewBufferString(f.compressed))
		gr, err = gzip.NewReader(b64)
		if err != nil {
			return
		}
		f.data, err = ioutil.ReadAll(gr)
	})
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (fs _escStaticFS) Open(name string) (http.File, error) {
	f, err := fs.prepare(name)
	if err != nil {
		return nil, err
	}
	return f.File()
}

func (dir _escDirectory) Open(name string) (http.File, error) {
	return dir.fs.Open(dir.name + name)
}

func (f *_escFile) File() (http.File, error) {
	type httpFile struct {
		*bytes.Reader
		*_escFile
	}
	return &httpFile{
		Reader:   bytes.NewReader(f.data),
		_escFile: f,
	}, nil
}

func (f *_escFile) Close() error {
	return nil
}

func (f *_escFile) Readdir(count int) ([]os.FileInfo, error) {
	if !f.isDir {
		return nil, fmt.Errorf(" escFile.Readdir: '%s' is not directory", f.name)
	}

	fis, ok := _escDirs[f.local]
	if !ok {
		return nil, fmt.Errorf(" escFile.Readdir: '%s' is directory, but we have no info about content of this dir, local=%s", f.name, f.local)
	}
	limit := count
	if count <= 0 || limit > len(fis) {
		limit = len(fis)
	}

	if len(fis) == 0 && count > 0 {
		return nil, io.EOF
	}

	return fis[0:limit], nil
}

func (f *_escFile) Stat() (os.FileInfo, error) {
	return f, nil
}

func (f *_escFile) Name() string {
	return f.name
}

func (f *_escFile) Size() int64 {
	return f.size
}

func (f *_escFile) Mode() os.FileMode {
	return 0
}

func (f *_escFile) ModTime() time.Time {
	return time.Unix(f.modtime, 0)
}

func (f *_escFile) IsDir() bool {
	return f.isDir
}

func (f *_escFile) Sys() interface{} {
	return f
}

// FS returns a http.Filesystem for the embedded assets. If useLocal is true,
// the filesystem's contents are instead used.
func FS(useLocal bool) http.FileSystem {
	if useLocal {
		return _escLocal
	}
	return _escStatic
}

// Dir returns a http.Filesystem for the embedded assets on a given prefix dir.
// If useLocal is true, the filesystem's contents are instead used.
func Dir(useLocal bool, name string) http.FileSystem {
	if useLocal {
		return _escDirectory{fs: _escLocal, name: name}
	}
	return _escDirectory{fs: _escStatic, name: name}
}

// FSByte returns the named file from the embedded assets. If useLocal is
// true, the filesystem's contents are instead used.
func FSByte(useLocal bool, name string) ([]byte, error) {
	if useLocal {
		f, err := _escLocal.Open(name)
		if err != nil {
			return nil, err
		}
		b, err := ioutil.ReadAll(f)
		_ = f.Close()
		return b, err
	}
	f, err := _escStatic.prepare(name)
	if err != nil {
		return nil, err
	}
	return f.data, nil
}

// FSMustByte is the same as FSByte, but panics if name is not present.
func FSMustByte(useLocal bool, name string) []byte {
	b, err := FSByte(useLocal, name)
	if err != nil {
		panic(err)
	}
	return b
}

// FSString is the string version of FSByte.
func FSString(useLocal bool, name string) (string, error) {
	b, err := FSByte(useLocal, name)
	return string(b), err
}

// FSMustString is the string version of FSMustByte.
func FSMustString(useLocal bool, name string) string {
	return string(FSMustByte(useLocal, name))
}

var _escData = map[string]*_escFile{

	"/assets/milligram.css": {
		name:    "milligram.css",
		local:   "assets/milligram.css",
		size:    10146,
		modtime: 1485417728,
		compressed: `
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
Gr+ENXjY/DsAAP//LNrHeqInAAA=
`,
	},

	"/assets/milligram.css.map": {
		name:    "milligram.css.map",
		local:   "assets/milligram.css.map",
		size:    14198,
		modtime: 1485417728,
		compressed: `
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
`,
	},

	"/tpl/tpl.gohtml": {
		name:    "tpl.gohtml",
		local:   "tpl/tpl.gohtml",
		size:    2691,
		modtime: 1565649698,
		compressed: `
H4sIAAAAAAAC/5RVS4/rJhTe51dQ1o3pvbOpKmzptlWlGY1mqnmoa2xOYjQYIownSZH/ewXY8SNOJvUK
Dud85/3ZOQ4boQBhyY66sbhtVwghRH/iurDHHaDSVjKLMn9EkqltikHhXgiMx2O4WmElZL9r/cHk8V8w
lETJoCGF+kAGZIpre5RQlwAWo9LAJsWE1TXYmlRCSrE1rEqKusbIB5JiCwdL/J2M0ALGcPdfUjGhUFJo
ZZlQYH7uJT5UMMhNtP1XscN6L7gtf0O//mKgmii0gzMy8kbJkDnNNT9mq3jm4hMVktV1ir1bPAq2C6B7
PQWIohxP06Dlt4yyvjB4UlOWUVJ+GyGTCDFInEPJH1pZUBb1PSVcfPZRkhhyZxx67Bwo3rar1TAUQnE4
nGaihsIKrc7in6T4PQRaU1J+H0cjNigJD207zdKyfN7AKJ8O1iA358LOILv/kxJbXn5/86N4XeVHY0tt
rus8MrVt2PYCEiVLIXrdCwkNnRh/zhmmtrBctS+rER/5aIByD0OcS+552+KsO8RRsvw6inNJKJ3Xv0E3
1vBG5b6Y19SXS9rP64L2eUkpmQ2acyBrmE8jVNmTRqHk6C/dKE4JVBOjk0tKun3oV+qm9bjLfnCOnmAf
nFBS3o1eN9pUqAJbap7iv59f3zBiAbPvH0agisiGbLeTomD+lRzW+/1+7a3XjZGgCs2Bz+kkLP9ZrSTL
QaKNNilujMTZ+8sjJUG4oCzUrrEjNsZ9rsG1T9hoiZHgEQwpVkF33ElWQKklB5Ninzp6f3mch0jOYqR5
Y61Wnc+6ySsxeM2tQrlV650RFTNHnL2Gd0qi0ZgefYDZvG0LhPcpYL/2pf5fpMflvNY2cBG3c3kYeZ9+
t32cd9MzNg09WLI9bXMP8m5kt8zDNa70InDHf9fCOu35IkDPjtcQhu1fhBjI8xrImBUWYR50jl4ts029
DHSZTx90Hg0vcKpzSdvS3JDbGCfGN7rJ+W/Pezv77ZV3WZBPGWBxmvqUn5oqB4P0Bv2jDV/Ie1TEgJ1E
i+dN0F8s5akrWm2htgH5BuBO3Wv3sLOayC9Jsxf/FwAA//8S2veJgwoAAA==
`,
	},

	"/assets": {
		name:  "assets",
		local: `assets`,
		isDir: true,
	},

	"/tpl": {
		name:  "tpl",
		local: `tpl`,
		isDir: true,
	},
}

var _escDirs = map[string][]os.FileInfo{

	"assets": {
		_escData["/assets/milligram.css"],
		_escData["/assets/milligram.css.map"],
	},

	"tpl": {
		_escData["/tpl/tpl.gohtml"],
	},
}

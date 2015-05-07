package yara

// #include <stdio.h>
// #include <yara.h>
// #include "cgo.h"
import "C"

import (
	"fmt"
	"log"
	"unsafe"
)

var callback = (C.YR_CALLBACK_FUNC)(unsafe.Pointer(C.callback))

// Callback is a callback that gets called during a scan with a matching rule.
type Callback func(*Rule) CallbackStatus

// CallbackStatus is flag to indicate to libyara if it should continue or abort a scanning
// process.
type CallbackStatus int

const (
	// Coninue tells libyara to continue the scanning.
	Continue = CallbackStatus(C.CALLBACK_CONTINUE)

	// Abort tells libyara to abort the scanning.
	Abort = CallbackStatus(C.CALLBACK_ABORT)

	// Fail tells libyara that an error occured and it should abort the scanning.
	Fail = CallbackStatus(C.CALLBACK_ERROR)
)

// Error is an libyara error.
type Error int

// Error returns an human readable repsentation of the libyara error.
func (e Error) Error() string {
	return fmt.Sprintf("libyara: %d", e)
}

func init() {
	code := C.yr_initialize()
	if code != C.ERROR_SUCCESS {
		log.Fatalf("failed to initialize libyara!")
	}
}

func Finalize() error {
	code := C.yr_finalize()
	if code != C.ERROR_SUCCESS {
		return Error(code)
	}

	return nil
}

type Compiler struct {
	handle *C.YR_COMPILER
}

func NewCompiler() (*Compiler, error) {
	var handle *C.YR_COMPILER
	code := C.yr_compiler_create(&handle)
	if code != C.ERROR_SUCCESS {
		return nil, Error(code)
	}

	return &Compiler{handle}, nil
}

func (c *Compiler) Destroy() {
	C.yr_compiler_destroy(c.handle)
}

func (c *Compiler) AddFile(ns, path string) error {
	cpath := C.CString(path)
	cmode := C.CString("r")
	cns := C.CString(ns)

	defer C.free(unsafe.Pointer(cpath))
	defer C.free(unsafe.Pointer(cmode))
	defer C.free(unsafe.Pointer(cns))

	fd := C.fopen(cpath, cmode)
	if fd == nil {
		return fmt.Errorf("libyara: failed to open %q", path)
	}

	defer C.fclose(fd)

	errors := C.yr_compiler_add_file(c.handle, fd, nil, cpath)
	if errors > 0 {
		return fmt.Errorf("libyara: failed to compile %q", path)
	}

	return nil
}

func (c *Compiler) AddString(ns, rule string) error {
	cns := C.CString(ns)
	crule := C.CString(rule)
	errors := C.yr_compiler_add_string(c.handle, crule, cns)
	C.free(unsafe.Pointer(crule))
	C.free(unsafe.Pointer(cns))

	if errors > 0 {
		return fmt.Errorf("libyara: failed to compile rule")
	}

	return nil
}

func (c *Compiler) Rules() (*Rules, error) {
	var handle *C.YR_RULES
	code := C.yr_compiler_get_rules(c.handle, &handle)
	if code != C.ERROR_SUCCESS {
		return nil, Error(code)
	}

	return &Rules{
		handle: handle,
	}, nil
}

type Rules struct {
	handle *C.YR_RULES
}

func LoadFromFile(path string) (*Rules, error) {
	var handle *C.YR_RULES

	cpath := C.CString(path)
	code := C.yr_rules_load(cpath, &handle)
	C.free(unsafe.Pointer(cpath))

	if code != C.ERROR_SUCCESS {
		return nil, Error(code)
	}

	return &Rules{
		handle: handle,
	}, nil
}

func (r *Rules) Destroy() {
	C.yr_rules_destroy(r.handle)
}

func (r *Rules) Save(path string) error {
	cpath := C.CString(path)
	code := C.yr_rules_save(r.handle, cpath)
	C.free(unsafe.Pointer(cpath))

	if code != C.ERROR_SUCCESS {
		return Error(code)
	}

	return nil
}

func (r *Rules) ScanMemory(buffer []byte, fn Callback) error {
	data := (*C.uint8_t)(unsafe.Pointer(&buffer[0]))
	size := C.size_t(len(buffer))

	code := C.yr_rules_scan_mem(r.handle, data, size, 0, callback, *(*unsafe.Pointer)(unsafe.Pointer(&fn)), 0)

	if code != C.ERROR_SUCCESS {
		return Error(code)
	}

	return nil
}

func (r *Rules) ScanFile(path string, fn Callback) error {
	cpath := C.CString(path)
	code := C.yr_rules_scan_file(r.handle, cpath, 0, callback, *(*unsafe.Pointer)(unsafe.Pointer(&fn)), 0)
	C.free(unsafe.Pointer(cpath))

	if code != C.ERROR_SUCCESS {
		return Error(code)
	}

	return nil
}

type Rule struct {
	Identifier string
	Tags       []string
	Metadata   map[string]string
}

func NewRule() *Rule {
	return &Rule{
		Metadata: make(map[string]string),
	}
}

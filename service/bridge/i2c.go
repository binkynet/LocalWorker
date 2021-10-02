// Copyright 2020 Ewout Prangsma
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Author Ewout Prangsma
//

package bridge

import (
	"fmt"
	"os"
	"sync"
	"syscall"
	"unsafe"

	"github.com/pkg/errors"
)

const (
	// From  /usr/include/linux/i2c-dev.h:
	// ioctl signals
	I2C_SLAVE = 0x0703
	I2C_FUNCS = 0x0705
	I2C_SMBUS = 0x0720
	// Read/write markers
	I2C_SMBUS_READ  = 1
	I2C_SMBUS_WRITE = 0

	// From  /usr/include/linux/i2c.h:
	// Adapter functionality
	I2C_FUNC_SMBUS_QUICK            = 0x00010000
	I2C_FUNC_SMBUS_READ_BYTE        = 0x00020000
	I2C_FUNC_SMBUS_WRITE_BYTE       = 0x00040000
	I2C_FUNC_SMBUS_READ_BYTE_DATA   = 0x00080000
	I2C_FUNC_SMBUS_WRITE_BYTE_DATA  = 0x00100000
	I2C_FUNC_SMBUS_READ_WORD_DATA   = 0x00200000
	I2C_FUNC_SMBUS_WRITE_WORD_DATA  = 0x00400000
	I2C_FUNC_SMBUS_READ_BLOCK_DATA  = 0x01000000
	I2C_FUNC_SMBUS_WRITE_BLOCK_DATA = 0x02000000
	// Transaction types
	I2C_SMBUS_QUICK            = 0
	I2C_SMBUS_BYTE             = 1
	I2C_SMBUS_BYTE_DATA        = 2
	I2C_SMBUS_WORD_DATA        = 3
	I2C_SMBUS_PROC_CALL        = 4
	I2C_SMBUS_BLOCK_DATA       = 5
	I2C_SMBUS_I2C_BLOCK_BROKEN = 6
	I2C_SMBUS_BLOCK_PROC_CALL  = 7 /* SMBus 2.0 */
	I2C_SMBUS_I2C_BLOCK_DATA   = 8 /* SMBus 2.0 */
)

type i2cSmbusIoctlData struct {
	readWrite byte
	command   byte
	size      uint32
	data      uintptr
}

type I2CBus interface {
	Close() (err error)
	ReadByteReg(addr byte, reg uint8) (uint8, error)
	WriteByteReg(addr byte, reg uint8, val uint8) (err error)
	// DetectSlaveAddresses probes the bus to detect available addresses.
	DetectSlaveAddresses() []byte
}

type i2cBus struct {
	mutex sync.Mutex
	file  *os.File
	funcs uint64 // adapter functionality mask
}

// NewI2cDevice returns an io.ReadWriteCloser with the proper ioctrl given
// an i2c bus location.
func NewI2cDevice(location string) (I2CBus, error) {
	d := &i2cBus{}

	var err error
	if d.file, err = os.OpenFile(location, os.O_RDWR, os.ModeExclusive); err != nil {
		return nil, err
	}
	if err := d.queryFunctionality(); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *i2cBus) queryFunctionality() (err error) {
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		d.file.Fd(),
		I2C_FUNCS,
		uintptr(unsafe.Pointer(&d.funcs)),
	)

	if errno != 0 {
		err = fmt.Errorf("Querying functionality failed with syscall.Errno %v", errno)
	}
	return
}

func (d *i2cBus) setAddress(address byte) (err error) {
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		d.file.Fd(),
		I2C_SLAVE,
		uintptr(address),
	)

	if errno != 0 {
		err = fmt.Errorf("Setting address failed with syscall.Errno %v", errno)
	}

	return
}

func (d *i2cBus) Close() (err error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	return d.file.Close()
}

func (d *i2cBus) DetectAddress(addr byte) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if err := d.setAddress(addr); err != nil {
		return errors.Wrap(err, "setAddress failed")
	}
	err := d.quick()
	if err != nil {
		return errors.Wrap(err, "quick failed")
	}
	return nil
}

func (d *i2cBus) ReadByteReg(addr byte, reg uint8) (uint8, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if err := d.setAddress(addr); err != nil {
		return 0, errors.Wrap(err, "setAddress failed")
	}
	val, err := d.readByteData(reg)
	if err != nil {
		return 0, errors.Wrap(err, "readByteData failed")
	}
	return val, nil
}

func (d *i2cBus) WriteByteReg(addr byte, reg uint8, val uint8) (err error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if err := d.setAddress(addr); err != nil {
		return errors.Wrap(err, "setAddress failed")
	}
	if err := d.writeByteData(reg, val); err != nil {
		return errors.Wrap(err, "writeByteData failed")
	}
	return nil
}

func (d *i2cBus) quick() (err error) {
	if d.funcs&I2C_FUNC_SMBUS_QUICK == 0 {
		return fmt.Errorf("SMBus quick not supported")
	}

	err = d.smbusAccess(I2C_SMBUS_WRITE, 0, I2C_SMBUS_QUICK, uintptr(0))
	return err
}

func (d *i2cBus) readByte() (val byte, err error) {
	if d.funcs&I2C_FUNC_SMBUS_READ_BYTE == 0 {
		return 0, fmt.Errorf("SMBus read byte not supported")
	}

	var data uint8
	err = d.smbusAccess(I2C_SMBUS_READ, 0, I2C_SMBUS_BYTE, uintptr(unsafe.Pointer(&data)))
	return data, err
}

func (d *i2cBus) readByteData(reg uint8) (val uint8, err error) {
	if d.funcs&I2C_FUNC_SMBUS_READ_BYTE_DATA == 0 {
		return 0, fmt.Errorf("SMBus read byte data not supported")
	}

	var data uint8
	err = d.smbusAccess(I2C_SMBUS_READ, reg, I2C_SMBUS_BYTE_DATA, uintptr(unsafe.Pointer(&data)))
	return data, err
}

func (d *i2cBus) readWordData(reg uint8) (val uint16, err error) {
	if d.funcs&I2C_FUNC_SMBUS_READ_WORD_DATA == 0 {
		return 0, fmt.Errorf("SMBus read word data not supported")
	}

	var data uint16
	err = d.smbusAccess(I2C_SMBUS_READ, reg, I2C_SMBUS_WORD_DATA, uintptr(unsafe.Pointer(&data)))
	return data, err
}

func (d *i2cBus) writeByte(val byte) (err error) {
	if d.funcs&I2C_FUNC_SMBUS_WRITE_BYTE == 0 {
		return fmt.Errorf("SMBus write byte not supported")
	}

	err = d.smbusAccess(I2C_SMBUS_WRITE, val, I2C_SMBUS_BYTE, uintptr(0))
	return err
}

func (d *i2cBus) writeByteData(reg uint8, val uint8) (err error) {
	if d.funcs&I2C_FUNC_SMBUS_WRITE_BYTE_DATA == 0 {
		return fmt.Errorf("SMBus write byte data not supported")
	}

	var data = val
	err = d.smbusAccess(I2C_SMBUS_WRITE, reg, I2C_SMBUS_BYTE_DATA, uintptr(unsafe.Pointer(&data)))
	return err
}

func (d *i2cBus) writeWordData(reg uint8, val uint16) (err error) {
	if d.funcs&I2C_FUNC_SMBUS_WRITE_WORD_DATA == 0 {
		return fmt.Errorf("SMBus write word data not supported")
	}

	var data = val
	err = d.smbusAccess(I2C_SMBUS_WRITE, reg, I2C_SMBUS_WORD_DATA, uintptr(unsafe.Pointer(&data)))
	return err
}

func (d *i2cBus) writeBlockData(reg uint8, data []byte) (err error) {
	if len(data) > 32 {
		return fmt.Errorf("Writing blocks larger than 32 bytes (%v) not supported", len(data))
	}

	buf := make([]byte, len(data)+1)
	copy(buf[1:], data)
	buf[0] = reg

	n, err := d.file.Write(buf)

	if err != nil {
		return err
	}

	if n != len(buf) {
		return fmt.Errorf("Write to device truncated, %v of %v written", n, len(buf))
	}

	return nil
}

// Read implements the io.ReadWriteCloser method by direct I2C read operations.
func (d *i2cBus) read(b []byte) (n int, err error) {
	return d.file.Read(b)
}

// Write implements the io.ReadWriteCloser method by direct I2C write operations.
func (d *i2cBus) write(b []byte) (n int, err error) {
	return d.file.Write(b)
}

func (d *i2cBus) smbusAccess(readWrite byte, command byte, size uint32, data uintptr) error {
	smbus := &i2cSmbusIoctlData{
		readWrite: readWrite,
		command:   command,
		size:      size,
		data:      data,
	}

	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		d.file.Fd(),
		I2C_SMBUS,
		uintptr(unsafe.Pointer(smbus)),
	)

	if errno != 0 {
		return fmt.Errorf("Failed with syscall.Errno %v", errno)
	}

	return nil
}

// DetectSlaveAddresses probes the bus to detect available addresses.
func (d *i2cBus) DetectSlaveAddresses() []byte {
	var result []byte
	for addr := 1; addr < 128; addr++ {
		if err := d.DetectAddress(byte(addr)); err == nil {
			result = append(result, byte(addr))
		}
	}
	return result
}

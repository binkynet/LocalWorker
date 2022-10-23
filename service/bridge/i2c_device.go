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

	aerr "github.com/ewoutp/go-aggregate-error"
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
	I2C_FUNC_SMBUS_READ_I2C_BLOCK   = 0x04000000 /* I2C-like block xfer  */
	I2C_FUNC_SMBUS_WRITE_I2C_BLOCK  = 0x08000000 /* w/ 1-byte reg. addr. */
	I2C_FUNC_SMBUS_HOST_NOTIFY      = 0x10000000 /* SMBus 2.0 or later */

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

type i2cDevice struct {
	bus     *i2cBus
	address uint8
	mutex   sync.Mutex
	file    *os.File
	funcs   uint64 // adapter functionality mask
}

// newI2CDevice returns accessors the the I2C address at the given location & address.
func newI2CDevice(bus *i2cBus, location string, address uint8) (*i2cDevice, error) {
	d := &i2cDevice{
		bus:     bus,
		address: address,
	}

	var err error
	if d.file, err = os.OpenFile(location, os.O_RDWR, os.ModeExclusive); err != nil {
		return nil, err
	}
	if err := d.queryFunctionality(); err != nil {
		return nil, err
	}
	if err := d.setAddress(address); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *i2cDevice) queryFunctionality() (err error) {
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

func (d *i2cDevice) setAddress(address byte) (err error) {
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		d.file.Fd(),
		I2C_SLAVE,
		uintptr(address),
	)

	if errno != 0 {
		err = fmt.Errorf("Setting address (0x%0x) failed with syscall.Errno %v", d.address, errno)
	}

	return
}

func (d *i2cDevice) Close() (err error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	var ae aerr.AggregateError
	if err := d.file.Close(); err != nil {
		ae.Add(err)
	}
	if err := d.bus.closeDevice(d.address); err != nil {
		ae.Add(err)
	}

	return ae.AsError()
}

func (d *i2cDevice) DetectDevice() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	err := d.quick()
	if err != nil {
		return errors.Wrap(err, "quick failed")
	}
	return nil
}

func (d *i2cDevice) ReadByteReg(reg uint8) (uint8, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	val, err := d.readByteData(reg)
	if err != nil {
		return 0, errors.Wrapf(err, "readByteData[0x%0x](0x%0x) failed", d.address, reg)
	}
	return val, nil
}

func (d *i2cDevice) WriteByteReg(reg uint8, val uint8) (err error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if err := d.writeByteData(reg, val); err != nil {
		return errors.Wrapf(err, "writeByteData[0x%0x](0x%0x, 0x%0x) failed", d.address, reg, val)
	}
	return nil
}

// Read a byte from device
func (d *i2cDevice) ReadByte() (uint8, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	val, err := d.readByte()
	if err != nil {
		return 0, errors.Wrapf(err, "readByte[0x%0x] failed", d.address)
	}
	return val, nil
}

// Write a byte to device
func (d *i2cDevice) WriteByte(val uint8) (err error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if err := d.writeByte(val); err != nil {
		return errors.Wrapf(err, "writeByte[0x%0x](0x%0x) failed", d.address, val)
	}
	return nil
}

// Read a 16-bits word to the device
// S Addr Wr [A] Comm [A] Sr Addr Rd [A] [DataLow] A [DataHigh] NA P
func (d *i2cDevice) ReadWordData(reg uint8) (val uint16, err error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	data, err := d.readWordData(reg)
	if err != nil {
		return 0, errors.Wrapf(err, "readWordData[0x%0x] failed", d.address, data)
	}
	return data, nil
}

// Write a 16-bits word to the device
// S Addr Wr [A] Comm [A] DataLow [A] DataHigh [A] P
func (d *i2cDevice) WriteWordData(reg uint8, data uint16) (err error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if err := d.writeWordData(reg, data); err != nil {
		return errors.Wrapf(err, "writeWordData[0x%0x](0x%0x) failed", d.address, data)
	}
	return nil
}

// Read a block of data (without count) to the device
// S Addr Wr [A] Comm [A] Sr Addr Rd [A] [Data] A [Data] A ... A [Data] NA P
func (d *i2cDevice) ReadI2CBlock(reg uint8, data []byte) (err error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if err := d.readI2CBlock(reg, data); err != nil {
		return errors.Wrapf(err, "readI2CBlock[0x%0x](0x%0x, ...) failed", d.address, reg)
	}
	return nil
}

// Write a block of data (without count) to the device
// S Addr Wr [A] Comm [A] Data [A] Data [A] ... [A] Data [A] P
func (d *i2cDevice) WriteI2CBlock(reg uint8, data []byte) (err error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if err := d.writeI2CBlock(reg, data); err != nil {
		return errors.Wrapf(err, "writeI2CBlock[0x%0x](0x%0x, %0x) failed", d.address, reg, data)
	}
	return nil
}

// Read a block of data directly from the device (/dev/...)
func (d *i2cDevice) ReadDevice(data []byte) (err error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	n, err := d.read(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return fmt.Errorf("expected to read %d bytes, actual written bytes is %d", len(data), n)
	}
	return nil
}

// Write a block of data directly to the device (/dev/...)
func (d *i2cDevice) WriteDevice(data []byte) (err error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	n, err := d.write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return fmt.Errorf("expected to write %d bytes, actual written bytes is %d", len(data), n)
	}
	return nil
}

func (d *i2cDevice) quick() (err error) {
	if d.funcs&I2C_FUNC_SMBUS_QUICK == 0 {
		return fmt.Errorf("SMBus quick not supported")
	}

	err = d.smbusAccess(I2C_SMBUS_WRITE, 0, I2C_SMBUS_QUICK, uintptr(0))
	return err
}

func (d *i2cDevice) readByte() (val byte, err error) {
	if d.funcs&I2C_FUNC_SMBUS_READ_BYTE == 0 {
		return 0, fmt.Errorf("SMBus read byte not supported")
	}

	var data uint8
	err = d.smbusAccess(I2C_SMBUS_READ, 0, I2C_SMBUS_BYTE, uintptr(unsafe.Pointer(&data)))
	return data, err
}

func (d *i2cDevice) readByteData(reg uint8) (val uint8, err error) {
	if d.funcs&I2C_FUNC_SMBUS_READ_BYTE_DATA == 0 {
		return 0, fmt.Errorf("SMBus read byte data not supported")
	}

	var data uint8
	err = d.smbusAccess(I2C_SMBUS_READ, reg, I2C_SMBUS_BYTE_DATA, uintptr(unsafe.Pointer(&data)))
	return data, err
}

func (d *i2cDevice) readWordData(reg uint8) (val uint16, err error) {
	if d.funcs&I2C_FUNC_SMBUS_READ_WORD_DATA == 0 {
		return 0, fmt.Errorf("SMBus read word data not supported")
	}

	var data uint16
	err = d.smbusAccess(I2C_SMBUS_READ, reg, I2C_SMBUS_WORD_DATA, uintptr(unsafe.Pointer(&data)))
	return data, err
}

func (d *i2cDevice) writeByte(val byte) (err error) {
	if d.funcs&I2C_FUNC_SMBUS_WRITE_BYTE == 0 {
		return fmt.Errorf("SMBus write byte not supported")
	}

	err = d.smbusAccess(I2C_SMBUS_WRITE, val, I2C_SMBUS_BYTE, uintptr(0))
	return err
}

func (d *i2cDevice) writeByteData(reg uint8, val uint8) (err error) {
	if d.funcs&I2C_FUNC_SMBUS_WRITE_BYTE_DATA == 0 {
		return fmt.Errorf("SMBus write byte data not supported")
	}

	var data = val
	err = d.smbusAccess(I2C_SMBUS_WRITE, reg, I2C_SMBUS_BYTE_DATA, uintptr(unsafe.Pointer(&data)))
	return err
}

func (d *i2cDevice) writeWordData(reg uint8, val uint16) (err error) {
	if d.funcs&I2C_FUNC_SMBUS_WRITE_WORD_DATA == 0 {
		return fmt.Errorf("SMBus write word data not supported")
	}

	var data = val
	err = d.smbusAccess(I2C_SMBUS_WRITE, reg, I2C_SMBUS_WORD_DATA, uintptr(unsafe.Pointer(&data)))
	return err
}

func (d *i2cDevice) writeBlockData(reg uint8, data []byte) (err error) {
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

func (d *i2cDevice) readI2CBlock(reg uint8, data []byte) (err error) {
	if d.funcs&I2C_FUNC_SMBUS_READ_I2C_BLOCK == 0 {
		return fmt.Errorf("SMBus read I2C block not supported")
	}

	buf := make([]byte, len(data)+1)
	buf[0] = byte(len(data))

	err = d.smbusAccess(I2C_SMBUS_READ, reg, I2C_FUNC_SMBUS_READ_I2C_BLOCK, uintptr(unsafe.Pointer(&data)))
	copy(data, buf[1:])
	return err
}

func (d *i2cDevice) writeI2CBlock(reg uint8, data []byte) (err error) {
	if d.funcs&I2C_FUNC_SMBUS_WRITE_I2C_BLOCK == 0 {
		return fmt.Errorf("SMBus write I2C block not supported")
	}

	buf := make([]byte, len(data)+1)
	copy(buf[1:], data)
	buf[0] = byte(len(data))

	err = d.smbusAccess(I2C_SMBUS_WRITE, reg, I2C_FUNC_SMBUS_WRITE_I2C_BLOCK, uintptr(unsafe.Pointer(&data)))
	return err
}

// Read implements the io.ReadWriteCloser method by direct I2C read operations.
func (d *i2cDevice) read(b []byte) (n int, err error) {
	return d.file.Read(b)
}

// Write implements the io.ReadWriteCloser method by direct I2C write operations.
func (d *i2cDevice) write(b []byte) (n int, err error) {
	return d.file.Write(b)
}

func (d *i2cDevice) smbusAccess(readWrite byte, command byte, size uint32, data uintptr) error {
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

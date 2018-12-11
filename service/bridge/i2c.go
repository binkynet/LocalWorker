package bridge

import (
	"fmt"
	"os"
	"sync"
	"syscall"
	"unsafe"
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
	I2C_FUNC_SMBUS_READ_BYTE        = 0x00020000
	I2C_FUNC_SMBUS_WRITE_BYTE       = 0x00040000
	I2C_FUNC_SMBUS_READ_BYTE_DATA   = 0x00080000
	I2C_FUNC_SMBUS_WRITE_BYTE_DATA  = 0x00100000
	I2C_FUNC_SMBUS_READ_WORD_DATA   = 0x00200000
	I2C_FUNC_SMBUS_WRITE_WORD_DATA  = 0x00400000
	I2C_FUNC_SMBUS_READ_BLOCK_DATA  = 0x01000000
	I2C_FUNC_SMBUS_WRITE_BLOCK_DATA = 0x02000000
	// Transaction types
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

type I2CBus struct {
	mutex sync.Mutex
	file  *os.File
	funcs uint64 // adapter functionality mask
}

// NewI2cDevice returns an io.ReadWriteCloser with the proper ioctrl given
// an i2c bus location.
func NewI2cDevice(location string) (d *I2CBus, err error) {
	d = &I2CBus{}

	if d.file, err = os.OpenFile(location, os.O_RDWR, os.ModeExclusive); err != nil {
		return
	}
	if err = d.queryFunctionality(); err != nil {
		return
	}

	return
}

func (d *I2CBus) queryFunctionality() (err error) {
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

func (d *I2CBus) setAddress(address int) (err error) {
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		d.file.Fd(),
		I2C_SLAVE,
		uintptr(byte(address)),
	)

	if errno != 0 {
		err = fmt.Errorf("Setting address failed with syscall.Errno %v", errno)
	}

	return
}

func (d *I2CBus) Close() (err error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	return d.file.Close()
}

func (d *I2CBus) ReadByteReg(addr byte, reg uint8) (uint8, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if err := d.setAddress(int(addr)); err != nil {
		return 0, maskAny(err)
	}
	val, err := d.readByteData(reg)
	if err != nil {
		return 0, maskAny(err)
	}
	return val, nil
}

func (d *I2CBus) WriteByteReg(addr byte, reg uint8, val uint8) (err error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if err := d.setAddress(int(addr)); err != nil {
		return maskAny(err)
	}
	if err := d.writeByteData(reg, val); err != nil {
		return maskAny(err)
	}
	return nil
}

func (d *I2CBus) readByte() (val byte, err error) {
	if d.funcs&I2C_FUNC_SMBUS_READ_BYTE == 0 {
		return 0, fmt.Errorf("SMBus read byte not supported")
	}

	var data uint8
	err = d.smbusAccess(I2C_SMBUS_READ, 0, I2C_SMBUS_BYTE, uintptr(unsafe.Pointer(&data)))
	return data, err
}

func (d *I2CBus) readByteData(reg uint8) (val uint8, err error) {
	if d.funcs&I2C_FUNC_SMBUS_READ_BYTE_DATA == 0 {
		return 0, fmt.Errorf("SMBus read byte data not supported")
	}

	var data uint8
	err = d.smbusAccess(I2C_SMBUS_READ, reg, I2C_SMBUS_BYTE_DATA, uintptr(unsafe.Pointer(&data)))
	return data, err
}

func (d *I2CBus) readWordData(reg uint8) (val uint16, err error) {
	if d.funcs&I2C_FUNC_SMBUS_READ_WORD_DATA == 0 {
		return 0, fmt.Errorf("SMBus read word data not supported")
	}

	var data uint16
	err = d.smbusAccess(I2C_SMBUS_READ, reg, I2C_SMBUS_WORD_DATA, uintptr(unsafe.Pointer(&data)))
	return data, err
}

func (d *I2CBus) writeByte(val byte) (err error) {
	if d.funcs&I2C_FUNC_SMBUS_WRITE_BYTE == 0 {
		return fmt.Errorf("SMBus write byte not supported")
	}

	err = d.smbusAccess(I2C_SMBUS_WRITE, val, I2C_SMBUS_BYTE, uintptr(0))
	return err
}

func (d *I2CBus) writeByteData(reg uint8, val uint8) (err error) {
	if d.funcs&I2C_FUNC_SMBUS_WRITE_BYTE_DATA == 0 {
		return fmt.Errorf("SMBus write byte data not supported")
	}

	var data = val
	err = d.smbusAccess(I2C_SMBUS_WRITE, reg, I2C_SMBUS_BYTE_DATA, uintptr(unsafe.Pointer(&data)))
	return err
}

func (d *I2CBus) writeWordData(reg uint8, val uint16) (err error) {
	if d.funcs&I2C_FUNC_SMBUS_WRITE_WORD_DATA == 0 {
		return fmt.Errorf("SMBus write word data not supported")
	}

	var data = val
	err = d.smbusAccess(I2C_SMBUS_WRITE, reg, I2C_SMBUS_WORD_DATA, uintptr(unsafe.Pointer(&data)))
	return err
}

func (d *I2CBus) writeBlockData(reg uint8, data []byte) (err error) {
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
func (d *I2CBus) read(b []byte) (n int, err error) {
	return d.file.Read(b)
}

// Write implements the io.ReadWriteCloser method by direct I2C write operations.
func (d *I2CBus) write(b []byte) (n int, err error) {
	return d.file.Write(b)
}

func (d *I2CBus) smbusAccess(readWrite byte, command byte, size uint32, data uintptr) error {
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
func (d *I2CBus) DetectSlaveAddresses() []byte {
	var result []byte
	for addr := 1; addr < 256; addr++ {
		if _, err := d.ReadByteReg(byte(addr), 0); err == nil {
			result = append(result, byte(addr))
		}
	}
	return result
}

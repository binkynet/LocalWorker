package devices

import (
	"strconv"
	"strings"
)

// parseAddress parses a string containing a numeric address.
func parseAddress(addr string) (int, error) {
	if strings.HasPrefix(addr, "0x") || strings.HasPrefix(addr, "0X") {
		addr = addr[2:]
		result, err := strconv.ParseUInt(addr, 16, 32)
		if err != nil {
			return 0, maskAny(err)
		}
		return int(result), nil
	}
	result, err := strconv.ParseUInt(addr, 10, 32)
	if err != nil {
		return 0, maskAny(err)
	}
	return int(result), nil
}

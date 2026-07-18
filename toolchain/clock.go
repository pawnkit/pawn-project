package toolchain

import "time"

// systemNow isolates wall-clock access for tests.
func systemNow() int64 {
	return time.Now().Unix()
}

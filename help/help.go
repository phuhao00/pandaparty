package help

import (
	"strconv"
)

// Helper functions
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func JoinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

// 新增的转换函数
// StringToInt32 converts string to int32
func StringToInt32(s string) int32 {
	if val, err := strconv.ParseInt(s, 10, 32); err == nil {
		return int32(val)
	}
	return 0
}

// Int32ToString converts int32 to string
func Int32ToString(i int32) string {
	return strconv.Itoa(int(i))
}

// StringToUint32 converts string to uint32
func StringToUint32(s string) uint32 {
	if val, err := strconv.ParseUint(s, 10, 32); err == nil {
		return uint32(val)
	}
	return 0
}

// Uint32ToString converts uint32 to string
func Uint32ToString(i uint32) string {
	return strconv.FormatUint(uint64(i), 10)
}

// ContainsInt32 checks if int32 slice contains item
func ContainsInt32(slice []int32, item int32) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ContainsInt64 checks if int64 slice contains item
func ContainsInt64(slice []int64, item int64) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// RemoveInt64 removes item from int64 slice
func RemoveInt64(slice []int64, item int64) []int64 {
	result := make([]int64, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

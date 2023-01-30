package common

import "fmt"

func ParseSize(s string) (int64, error) {
	var size int64
	var unit string
	n, err := fmt.Sscanf(s, "%d%s", &size, &unit)
	if err != nil {
		return 0, err
	}
	if n != 2 {
		return 0, fmt.Errorf("invalid size: %s", s)
	}
	switch unit {
	case "b", "B":
		return size, nil
	case "kb", "KB":
		return size * 1024, nil
	case "mb", "MB":
		return size * 1024 * 1024, nil
	case "gb", "GB":
		return size * 1024 * 1024 * 1024, nil
	default:
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}
}

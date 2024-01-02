package postgres

import (
	"fmt"
)

func atoi(s string) uint {
	var i uint
	_, err := fmt.Sscan(s, &i)
	if err != nil {
		return 0
	}
	return i
}

func itoa(i uint) string {
	return fmt.Sprint(i)
}

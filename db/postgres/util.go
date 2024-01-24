package postgres

import (
	"fmt"

	"github.com/suxatcode/learn-graph-poc-backend/db"
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

func mergeText(a, b db.Text) db.Text {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	r := make(db.Text, len(a)+len(b))
	for key, value := range a {
		r[key] = value
	}
	for key, value := range b {
		r[key] = value
	}
	return r
}

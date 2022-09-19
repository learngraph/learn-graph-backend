package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAll(t *testing.T) {
	for _, test := range []struct {
		Name  string
		Slice []interface{}
		Pred  func(t interface{}) bool
		Exp   bool
	}{
		{
			Name:  "empty slice returns true",
			Slice: []interface{}{},
			Pred:  func(i interface{}) bool { return true },
			Exp:   true,
		},
		{
			Name:  "bool slice, contains false",
			Slice: []interface{}{true, false, true},
			Pred:  func(i interface{}) bool { return i.(bool) },
			Exp:   false,
		},
		{
			Name:  "bool slice, only true",
			Slice: []interface{}{true, true, true},
			Pred:  func(i interface{}) bool { return i.(bool) },
			Exp:   true,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert.Equal(t, test.Exp, All(test.Slice, test.Pred))
		})
	}
}

var Slice123 = []interface{}{1, 2, 3}
var Slice1232 = []interface{}{1, 2, 3, 2}

func TestFindFirst(t *testing.T) {
	for _, test := range []struct {
		Name  string
		Slice []interface{}
		Pred  func(t interface{}) bool
		Exp   interface{}
	}{
		{
			Name:  "does not exist",
			Slice: []interface{}{1, 3, 4},
			Pred:  func(t interface{}) bool { return t.(int) == 2 },
			Exp:   nil,
		},
		{
			Name:  "exists once",
			Slice: Slice123,
			Pred:  func(t interface{}) bool { return t.(int) == 2 },
			Exp:   &Slice123[1],
		},
		{
			Name:  "exists once, finds first",
			Slice: Slice1232,
			Pred:  func(t interface{}) bool { return t.(int) == 2 },
			Exp:   &Slice123[1],
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			if test.Exp == nil {
				assert.Nil(t, FindFirst(test.Slice, test.Pred))
			} else {
				assert.Equal(t, test.Exp, FindFirst(test.Slice, test.Pred))
			}
		})
	}
}

func TestFindAll(t *testing.T) {
	for _, test := range []struct {
		Name  string
		Slice []int
		Pred  func(t int) bool
		Exp   []int
	}{
		{
			Name:  "does not exist",
			Slice: []int{1, 3, 4},
			Pred:  func(t int) bool { return t == 2 },
			Exp:   []int{},
		},
		{
			Name:  "finds one",
			Slice: []int{1, 3},
			Pred:  func(t int) bool { return t == 3 },
			Exp:   []int{3},
		},
		{
			Name:  "finds two",
			Slice: []int{3, 1, 9},
			Pred:  func(t int) bool { return t%3 == 0 },
			Exp:   []int{3, 9},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert.Equal(t, test.Exp, FindAll(test.Slice, test.Pred))
		})
	}
}

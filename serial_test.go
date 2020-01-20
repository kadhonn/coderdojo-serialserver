package main

import (
	"strconv"
	"testing"
)

func TestDoMapping1(t *testing.T) {
	callDoMapping(t, 5, 0, 100, 0, 100, 5)
}
func TestDoMapping12(t *testing.T) {
	callDoMapping(t, 0, 0, 100, 0, 100, 0)
}
func TestDoMapping13(t *testing.T) {
	callDoMapping(t, 100, 0, 100, 0, 100, 100)
}
func TestDoMapping2(t *testing.T) {
	//callDoMapping(t, 5, 0, 10, 0, 100, 50)
	for i := -100; i <= 100; i++ {
		print("i=")
		print(i)
		print(" ")
		b, _ := doMapping(i, -100, 100, 255, 1)
		println(b)
	}
}

func callDoMapping(t *testing.T, in int, inStart int, inEnd int, outStart int, outEnd int, wanted byte) {
	var out, err = doMapping(in, inStart, inEnd, outStart, outEnd)
	if err != nil {
		t.Error(err)
	}
	if out != wanted {
		t.Error("got " + strconv.Itoa(int(out)) + " wanted " + strconv.Itoa(int(wanted)))
	}
}

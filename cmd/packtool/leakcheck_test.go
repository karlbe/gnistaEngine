package main

import "testing"

func TestSamePicture(t *testing.T) {
	a := []uint8{0, 1, 2, 3, 0, 1, 2, 3}
	recoloured := []uint8{9, 8, 7, 6, 9, 8, 7, 6}
	if !samePicture(a, recoloured, nil, nil, 4) {
		t.Error("a recoloured copy is the same picture")
	}
	merged := []uint8{9, 8, 7, 7, 9, 8, 7, 7}
	if samePicture(a, merged, nil, nil, 4) {
		t.Error("two colours became one: not the same")
	}
	other := []uint8{0, 1, 2, 3, 3, 2, 1, 0}
	if samePicture(a, other, nil, nil, 4) {
		t.Error("a different arrangement is not the same")
	}
	flat := make([]uint8, 8)
	if samePicture(flat, flat, nil, nil, 4) {
		t.Error("flat pictures are not compared")
	}
	m1 := []bool{true, true, true, true, false, false, true, true}
	m2 := []bool{true, true, true, true, true, false, true, true}
	if samePicture(a, recoloured, m1, m1, 3) != true || samePicture(a, recoloured, m1, m2, 3) {
		t.Error("masks must match")
	}
}

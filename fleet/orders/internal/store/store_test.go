package store

import "testing"

func TestCreateAndGet(t *testing.T) {
	s := New()
	o := s.Create("r1", "c1", nil, 1250, 250)
	got, err := s.Get(o.Id)
	if err != nil || got.TotalPence != 1250 {
		t.Fatalf("got %+v %v", got, err)
	}
}

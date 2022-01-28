package config

import (
	"reflect"
	"testing"
)

func TestGetConf(t *testing.T) {
	type TestSubConf struct {
		Name string
		Age  uint64
	}
	type TestConf struct {
		Config
		Name  string
		Age   uint64
		Child TestSubConf
	}
	conf := &TestConf{
		Name: "SincereXIA",
		Age:  22,
		Child: TestSubConf{
			Name: "XiongHC",
			Age:  22,
		},
	}

	var testConf TestConf
	Register(&testConf, "test.json")
	ReadAll()

	if !reflect.DeepEqual(&testConf, conf) {
		t.Errorf("config not equal")
	}
}

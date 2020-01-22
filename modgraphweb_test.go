package main

import "testing"

func TestRegexp(t *testing.T) {
	matches := nameRegexp.FindStringSubmatch("/view/5b9d37f5337909968412a123cfc00973")
	if len(matches) != 2 && matches[1] != "5b9d37f5337909968412a123cfc00973" {
		t.Error(matches)
	}
}

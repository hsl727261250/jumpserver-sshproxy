package jmsauth_test

import (
	"testing"

	"github.com/jumpserver/jumpserver-sshproxy/internal/jmsauth"
)

func TestParseLoadavg(t *testing.T) {
	load, err := jmsauth.ParseLoadavg("0.42 0.30 0.20 1/120 9\n")
	if err != nil {
		t.Fatal(err)
	}
	if load != 0.4 {
		t.Fatalf("load %v", load)
	}
	if _, err = jmsauth.ParseLoadavg(" \n"); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseMeminfo(t *testing.T) {
	text := "MemTotal: 1000 kB\nMemFree: 100 kB\nMemAvailable: 250 kB\n"
	used, err := jmsauth.ParseMeminfo(text)
	if err != nil {
		t.Fatal(err)
	}
	if used != 75 {
		t.Fatalf("used %v", used)
	}
	used, err = jmsauth.ParseMeminfo("MemTotal: 200 kB\nMemFree: 50 kB\n")
	if err != nil {
		t.Fatal(err)
	}
	if used != 75 {
		t.Fatalf("free fallback %v", used)
	}
	if _, err = jmsauth.ParseMeminfo("MemFree: 1 kB\n"); err == nil {
		t.Fatal("expected error")
	}
}

func TestDiskUsedPercent(t *testing.T) {
	used, err := jmsauth.DiskUsedPercent("/")
	if err != nil {
		t.Fatal(err)
	}
	if used < 0 || used > 100 {
		t.Fatalf("disk %v", used)
	}
}

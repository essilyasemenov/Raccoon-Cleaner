package main

import (
	"os"
	"os/exec"
	"testing"
)

func TestRunLockExcludesSecondHolder(t *testing.T) {
	root := t.TempDir()
	first, err := acquireRunLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := acquireRunLock(root); err == nil {
		second.Close()
		first.Close()
		t.Fatal("second holder acquired the same lock")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	third, err := acquireRunLock(root)
	if err != nil {
		t.Fatalf("lock remained held after close: %v", err)
	}
	if err := third.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRunLockExcludesOtherProcess(t *testing.T) {
	root := t.TempDir()
	lock, err := acquireRunLock(root)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	cmd := exec.Command(os.Args[0], "-test.run=^TestRunLockChild$")
	cmd.Env = append(os.Environ(), "CLEANER_LOCK_TEST_ROOT="+root)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("other process acquired lock or failed unexpectedly: %v, %s", err, output)
	}
}

func TestRunLockChild(t *testing.T) {
	root := os.Getenv("CLEANER_LOCK_TEST_ROOT")
	if root == "" {
		return
	}
	if lock, err := acquireRunLock(root); err == nil {
		lock.Close()
		t.Fatal("lock held by another process was acquired")
	}
}

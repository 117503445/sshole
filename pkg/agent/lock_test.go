//go:build !windows

package agent

import (
	"errors"
	"testing"
)

// TestAcquireInstanceLockExclusive 验证第二个实例拿不到锁。
func TestAcquireInstanceLockExclusive(t *testing.T) {
	release, err := AcquireInstanceLock()
	if err != nil {
		t.Fatalf("first AcquireInstanceLock: %v", err)
	}
	defer release()

	if _, err := AcquireInstanceLock(); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second AcquireInstanceLock err = %v, want ErrAlreadyRunning", err)
	}
}

// TestAcquireInstanceLockRelease 验证释放后可以被再次获取。
func TestAcquireInstanceLockRelease(t *testing.T) {
	release, err := AcquireInstanceLock()
	if err != nil {
		t.Fatalf("first AcquireInstanceLock: %v", err)
	}
	release()

	release2, err := AcquireInstanceLock()
	if err != nil {
		t.Fatalf("AcquireInstanceLock after release: %v", err)
	}
	release2()
}

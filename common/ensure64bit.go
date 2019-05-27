// +build amd64 arm64 arm64be ppc64 ppc64le mips64 mips64le s390x sparc64

package common

// sadly we assume int is 64bits in length, so we don't support 32bit builds because of that.
func ensure64bit() {}

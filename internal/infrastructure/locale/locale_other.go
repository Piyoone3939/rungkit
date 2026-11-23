//go:build !windows

package locale

// systemLocale has nothing beyond the environment variables on this system.
func systemLocale() string { return "" }

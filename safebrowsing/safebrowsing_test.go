package safebrowsing

import (
	"testing"
)

func ensureDB(t *testing.T) bool {

	if SafeBrowser != nil {
		return true
	}

	err := runDatabase()
	if err != nil {
		if err == ErrNoSafebrowsingAPIKey {
			t.Skip("no safebrowsing api key provided, skipping test")
			return false
		}
		t.Error("failed ensuring db: ", err)
		return false
	}

	return true
}

func TestLocalLookup(t *testing.T) {
	if !ensureDB(t) {
		return
	}

	threat, err := performLocalLookup("https://testsafebrowsing.appspot.com/s/malware.html")
	if err != nil {
		t.Error("error looking up test site: ", err)
	} else if threat == nil {
		t.Error("https://testsafebrowsing.appspot.com/s/malware.html shouldve been a threat")
	}

	threat, err = performLocalLookup("https://google.com")
	if err != nil {
		t.Error("error looking up google: ", err)
	} else if threat != nil {
		t.Error("google.com should not be a threat!")
	}
}

// func ensureRESTServer(t *testing.T) bool {
// 	if restServer != nil {
// 		return true
// 	}

// 	RunServer()
// 	return true
// }

// func TestRemoteLookup(t *testing.T) {
// 	if !ensureRESTServer(t) {
// 		return
// 	}

// 	threat, err := performRemoteLookup("https://testsafebrowsing.appspot.com/s/malware.html")
// 	if err != nil {
// 		t.Error("error looking up test site: ", err)
// 	} else if threat == nil {
// 		t.Error("https://testsafebrowsing.appspot.com/s/malware.html shouldve been a threat")
// 	}

// 	threat, err = performRemoteLookup("https://google.com")
// 	if err != nil {
// 		t.Error("error looking up google: ", err)
// 	} else if threat != nil {
// 		t.Error("google.com should not be a threat!")
// 	}
// }

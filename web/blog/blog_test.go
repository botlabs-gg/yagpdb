package blog

import (
	"testing"
)

func TestParsePost(t *testing.T) {
	const sample = `{
		"title": "TestPost",
		"Date": "5 Aug. 2017"
}
Post body herer
 
 - yaboi
 - hohoho`

	const rendered = `<p>Post body herer</p>

<ul>
<li>yaboi</li>
<li>hohoho</li>
</ul>
`

	p, err := parsePost([]byte(sample))
	if err != nil {
		t.Fatal("Failed parsing: ", err)
	}

	if string(p.RenderedBody) != rendered {
		t.Error("Incorred output, got: ", string(p.RenderedBody), "\nextected: ", rendered)
	}
}

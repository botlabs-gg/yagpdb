package discordgo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"reflect"
)

// MultipartBodyWithJSON returns the contentType and body for a discord request
// data  : The object to encode for payload_json in the multipart request
// files : Files to include in the request
func MultipartBodyWithJSON(data interface{}, files []*File) (requestContentType string, requestBody []byte, err error) {
	body := &bytes.Buffer{}
	bodywriter := multipart.NewWriter(body)

	payload, err := json.Marshal(data)
	if err != nil {
		return
	}

	var p io.Writer

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="payload_json"`)
	h.Set("Content-Type", "application/json")

	p, err = bodywriter.CreatePart(h)
	if err != nil {
		return
	}

	if _, err = p.Write(payload); err != nil {
		return
	}

	for i, file := range files {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file%d"; filename="%s"`, i, quoteEscaper.Replace(file.Name)))
		contentType := file.ContentType
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		h.Set("Content-Type", contentType)

		p, err = bodywriter.CreatePart(h)
		if err != nil {
			return
		}

		if _, err = io.Copy(p, file.Reader); err != nil {
			return
		}
	}

	err = bodywriter.Close()
	if err != nil {
		return
	}

	return bodywriter.FormDataContentType(), body.Bytes(), nil
}

func IsEmbedEmpty(embed *MessageEmbed) bool {
	return reflect.DeepEqual(embed, &MessageEmbed{})
}

func ValidateComplexMessageEmbeds(embeds []*MessageEmbed) []*MessageEmbed {
	totalNils := 0
	totalEmbeds := len(embeds)
	parsedEmbeds := make([]*MessageEmbed, 0, totalEmbeds)

	for _, e := range embeds {
		if e == nil || IsEmbedEmpty(e) {
			totalNils++
		} else {
			if e.Type != "" {
				e.Type = "rich"
			}
			parsedEmbeds = append(parsedEmbeds, e)
		}
	}
	if totalEmbeds > 0 && totalNils == totalEmbeds {
		return nil
	}

	return parsedEmbeds
}

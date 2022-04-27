dca [![Go report](http://goreportcard.com/badge/jonas747/dca)](http://goreportcard.com/report/jonas747/dca) [![Build Status](https://travis-ci.org/jonas747/dca.svg?branch=master)](https://travis-ci.org/jonas747/dca)
====
`dca` is a audio file format that uses opus packets and json metadata.

This package implements a decoder, encoder and a helper streamer for dca v0 and v1.

[Docs on GoDoc](https://godoc.org/github.com/jonas747/dca)

There's also a standalone command you can use [here](https://github.com/jonas747/dca/tree/master/cmd/dca)

Usage
===
Encoding
```go

// Encoding a file and saving it to disk
encodeSession := dca.EncodeFile("path/to/file.mp3", dca.StdEncodeOptions)
// Make sure everything is cleaned up, that for example the encoding process if any issues happened isnt lingering around
defer encodeSession.Cleanup()

output, err := os.Create("output.dca")
if err != nil {
    // Handle the error
}

io.Copy(output, encodeSession)
```

Decoding, the decoder automatically detects  dca version aswell as if metadata was available
```go
// inputReader is an io.Reader, like a file for example
decoder := dca.NewDecoder(inputReader)

for {
    frame, err := decoder.OpusFrame()
    if err != nil {
        if err != io.EOF {
            // Handle the error
        }
        
        break
    }
    
    // Do something with the frame, in this example were sending it to discord
    select{
        case voiceConnection.OpusSend <- frame:
        case <-time.After(time.Second):
            // We haven't been able to send a frame in a second, assume the connection is borked
            return
    }
}

```

Using the helper streamer, the streamer creates a pausable stream to Discord.
```go

// Source is an OpusReader, both EncodeSession and decoder implements opusreader
done := make(chan error)
streamer := dca.NewStreamer(source, voiceConnection, done)
err := <- done
if err != nil && err != io.EOF {
    // Handle the error
}

```

Using this [youtube-dl](https://www.github.com/rylio/ytdl) Go package, one can stream music to Discord from Youtube
```go
// Change these accordingly
options := dca.StdEncodeOptions
options.RawOutput = true
options.Bitrate = 96
options.Application = "lowdelay"

videoInfo, err := ytdl.GetVideoInfo(videoURL)
if err != nil {
    // Handle the error
}

format := videoInfo.Formats.Extremes(ytdl.FormatAudioBitrateKey, true)[0]
downloadURL, err := videoInfo.GetDownloadURL(format)
if err != nil {
    // Handle the error
}

encodingSession, err := dca.EncodeFile(downloadURL.String(), options)
if err != nil {
    // Handle the error
}
defer encodingSession.Cleanup()
    
done := make(chan error)    
dca.NewStream(encodingSession, voiceConnection, done)
err := <- done
if err != nil && err != io.EOF {
    // Handle the error
}
```

### Official Specifications
* [DCA Repo](https://github.com/bwmarrin/dca)
* [DCA1 specification draft](https://github.com/bwmarrin/dca/wiki/DCA1-specification-draft)

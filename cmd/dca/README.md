dca  
====

This is a command line tool for creating DCA files

If you are developing a library for use with Discord you can use this program
as a way to generate the opus audio data from any standard audio file.

You can also pipe the output of this program to create a .dca file for later use.

* See [Discordgo](https://github.com/bwmarrin/discordgo) for Discord API bindings in Go.
* See the [wiki](https://github.com/bwmarrin/dca/wiki/DCA1-specification-draft) for more information on the DCA1 standard.

Join [#go_discordgo](https://discord.gg/0SBTUU1wZTWT6sqd) Discord chat channel 
for support.

## Features
* Stereo Audio
* 48khz Sampling Rate
* 20ms / 1920 byte audio frame size
* Bit-rates from 8 kb/s to 128 kb/s
* Optimization setting for VoIP, Audio, and Low Delay audio


## Getting Started

### Installing

dca has been tested to compile on FreeBSD 10 (Go 1.5.1), OS X 10.10, Windows 10.

### Ubuntu 14.04.3 LTS

Provided by Uniquoooo

```
# basics
sudo apt-get update
# golang
mkdir $HOME/go
echo 'export GOPATH=$HOME/go' >> ~/.bashrc
echo 'export PATH=$PATH:$GOPATH/bin' >> ~/.bashrc
source ~/.bashrc
# ffmpeg
sudo add-apt-repository ppa:kirillshkrogalev/ffmpeg-next
sudo apt-get update
sudo apt-get install ffmpeg --yes
# install dca
go get github.com/jonas747/dca/cmd/dca
```

Note: If Go complains that GOPATH is not defined, try run `source ~/.bashrc` and then `go get github.com/jonas747/dca/cmd/dca`.

### Windows

```
Install Go for Windows
Install ffmpeg
Setup gopath to some empty folder (for example, I made mine C:\gopath)
go get github.com/jonas747/dca/cmd/dca
dca should now be built in %GOPATH%/bin
```

### OS X

Provided by Uniquoooo

This way uses Homebrew, download it from [here.](http://brew.sh/)

```
$ brew install ffmpeg --with-opus
$ brew install golang
$ go get github.com/jonas747/dca/cmd/dca
```


### Usage

```
Usage of ./dca:
  -aa string
        audio application can be voip, audio, or lowdelay (default "audio")
  -ab int
        audio encoding bitrate in kb/s can be 8 - 128 (default 64)
  -ac int
        audio channels (default 2)
  -ar int
        audio sampling rate (default 48000)
  -as int
        audio frame size can be 960 (20ms), 1920 (40ms), or 2880 (60ms) (default 960)
  -cf string
        format the cover art will be encoded with (default "jpeg")
  -i string
        infile (default "pipe:0")
  -vol int
        change audio volume (256=normal) (default 256)
```

You may also pipe audio audio into dca instead of providing an input file.


## Examples

See the example folder.


## Contributing

While contributions are always welcome - this code is in a very early and 
incomplete stage and massive changes, including entire re-writes, could still
happen.  In other words, probably not worth your time right now :)

## List of Discord APIs

See [this chart](https://abal.moe/Discord/Libraries.html) for a feature 
comparison and list of other Discord API libraries.


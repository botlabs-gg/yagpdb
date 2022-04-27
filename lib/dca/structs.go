package dca

// Base metadata struct
//
// https://github.com/bwmarrin/dca/issues/5#issuecomment-189713886
type Metadata struct {
	Dca      *DCAMetadata    `json:"dca"`
	Opus     *OpusMetadata   `json:"opus"`
	SongInfo *SongMetadata   `json:"info"`
	Origin   *OriginMetadata `json:"origin"`
	Extra    *ExtraMetadata  `json:"extra"`
}

// DCA metadata struct
//
// Contains the DCA version.
type DCAMetadata struct {
	Version int8             `json:"version"`
	Tool    *DCAToolMetadata `json:"tool"`
}

// DCA tool metadata struct
//
// Contains the Git revisions, commit author etc.
type DCAToolMetadata struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Url     string `json:"url"`
	Author  string `json:"author"`
}

// Song Information metadata struct
//
// Contains information about the song that was encoded.
type SongMetadata struct {
	Title    string  `json:"title"`
	Artist   string  `json:"artist"`
	Album    string  `json:"album"`
	Genre    string  `json:"genre"`
	Comments string  `json:"comments"`
	Cover    *string `json:"cover"`
}

// Origin information metadata struct
//
// Contains information about where the song came from,
// audio bitrate, channels and original encoding.
type OriginMetadata struct {
	Source   string `json:"source"`
	Bitrate  int    `json:"abr"`
	Channels int    `json:"channels"`
	Encoding string `json:"encoding"`
	Url      string `json:"url"`
}

// Opus metadata struct
//
// Contains information about how the file was encoded
// with Opus.
type OpusMetadata struct {
	Bitrate     int    `json:"abr"`
	SampleRate  int    `json:"sample_rate"`
	Application string `json:"mode"`
	FrameSize   int    `json:"frame_size"`
	Channels    int    `json:"channels"`
	VBR         bool   `json:"vbr"`
}

// Extra metadata struct
type ExtraMetadata struct{}

////////////////////////////////////////////////////////
/// FFprobe Structures
////////////////////////////////////////////////////////

type FFprobeMetadata struct {
	Format *FFprobeFormat `json:"format"`
}

type FFprobeFormat struct {
	FileName       string `json:"filename"`
	NumStreams     int    `json:"nb_streams"`
	NumPrograms    int    `json:"nb_programs"`
	FormatName     string `json:"format_name"`
	FormatLongName string `json:"format_long_name"`
	StartTime      string `json:"start_time"`
	Duration       string `json:"duration"`
	Size           string `json:"size"`
	Bitrate        string `json:"bit_rate"`
	ProbeScore     int    `json:"probe_score"`

	Tags *FFprobeTags `json:"tags"`
}

type FFprobeTags struct {
	Date        string `json:"date"`
	Track       string `json:"track"`
	Artist      string `json:"artist"`
	Genre       string `json:"genre"`
	Title       string `json:"title"`
	Album       string `json:"album"`
	Compilation string `json:"compilation"`
}

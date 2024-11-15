package transcoder

type (
	VideoCodecType int
	AudioCodecType int
)

const (
	H264 VideoCodecType = iota
	H265
)

const (
	AAC AudioCodecType = iota
)

type TranscoderConfig struct {
	ID         string
	FfmpegBin  string
	SourceHls  string
	Varients   []string
	VideoCodec VideoCodecType
	AudioCodec AudioCodecType
	CacheDir   string
}

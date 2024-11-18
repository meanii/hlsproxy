package transcoder

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/grafov/m3u8"
	"github.com/meanii/hlsproxy/config"
	"github.com/meanii/hlsproxy/internal/externalcmd"
	"go.uber.org/zap"
)

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

type Transcoder struct {
	ID                 string
	FfmpegBin          string
	SourceHls          string
	Varients           []string
	VideoCodec         VideoCodecType
	AudioCodec         AudioCodecType
	CacheDir           string
	VideoResolutionMap map[string]string
	MasterHls          string
}

func NewTranscoder(sourceHls string, ID string) *Transcoder {
	config := Transcoder{
		SourceHls: sourceHls,
		ID:        ID,
	}
	config.FfmpegBin = "/opt/homebrew/bin/ffmpeg"
	config.Varients = []string{"240p", "360p", "audio"}
	config.VideoResolutionMap = map[string]string{
		"240p":  "426x240",
		"360p":  "640x360",
		"480p":  "854x480",
		"720p":  "1280x720",
		"1080p": "1920x1080",
		"1440p": "2560x1440",
		"2160p": "3840x2160",
		"audio": "audio",
	}
	config.VideoCodec = H264
	config.AudioCodec = AAC
	return &config
}

func (t *Transcoder) Run() error {
	t.prepareOutputDir()
	cmdstring := t.generateCmdString()
	err := t.generateMasterHls()
	cmdrunnerpool := externalcmd.NewPool()
	_ = externalcmd.NewCmd(
		cmdrunnerpool, cmdstring, true, make(externalcmd.Environment), nil)
	return err
}

func (t *Transcoder) generateCmdString() string {
	prefix := fmt.Sprintf("%s -i \"%s\" -profile:v baseline -level 3.0 ", t.FfmpegBin, t.SourceHls)
	suffixtree := make([]string, 0)

	for _, varient := range t.Varients {
		resolution := t.getResolution(varient)
		if varient != "audio" {
			segmentFilename := "%03d.ts"
			cmd := fmt.Sprintf("-s %s -start_number 0 -hls_time 2 -hls_list_size 3 -hls_flags delete_segments+split_by_time -hls_segment_filename %s/%s/%s -b:v 500k -maxrate 500k -bufsize 1000k -f hls %s/%s/%s.m3u8 ",
				resolution,
				t.CacheDir,
				varient,
				segmentFilename,
				t.CacheDir,
				varient,
				varient,
			)
			suffixtree = append(suffixtree, cmd)
		}

		if varient == "audio" {
			segmentFilename := "%03d.ts"
			cmd := fmt.Sprintf("-map 0:a -start_number 0 -hls_time 2 -hls_list_size 3 -hls_flags delete_segments+split_by_time -hls_segment_filename %s/audio/%s -b:a 128k -f hls %s/audio/audio.m3u8 ",
				t.CacheDir,
				segmentFilename,
				t.CacheDir,
			)
			suffixtree = append(suffixtree, cmd)
		}

	}

	suffixtreeString := strings.Join(suffixtree, " ")
	cmdstring := fmt.Sprintf("%s %s", prefix, suffixtreeString)
	zap.S().Infof("generated cmd string: %s", cmdstring)

	return cmdstring
}

func (t *Transcoder) prepareOutputDir() {
	wd, _ := os.Getwd()
	t.CacheDir = path.Join(wd, config.GlobalConfigInstance.Config.Cache.Dirname, t.ID)
	zap.S().Infof("setting up output dir: %s", t.CacheDir)
	for _, varient := range t.Varients {
		varientPath := path.Join(t.CacheDir, varient)
		err := os.MkdirAll(varientPath, os.ModePerm)
		if err != nil {
			zap.S().Warnf("transcoder: failed to mkdir, Error: %s", err)
		}
	}
}

func (t *Transcoder) generateMasterHls() error {
	masterHls := m3u8.NewMasterPlaylist()

	for _, varient := range t.Varients {
		genrateVarient := m3u8.Variant{
			URI: fmt.Sprintf("%s/%s.m3u8", varient, varient),
			VariantParams: m3u8.VariantParams{
				Resolution: t.getResolution(varient),
			},
		}
		masterHls.Variants = append(masterHls.Variants, &genrateVarient)
	}

	zap.S().Infof("transcoder, generated master.m3u8 hls file, %s", masterHls.String())
	masterfilepath := path.Join(t.CacheDir, "playlist.m3u8")
	t.MasterHls = masterfilepath
	err := t.writeFile(masterfilepath, []byte(masterHls.String()))
	return err
}

func (t *Transcoder) writeFile(filepath string, data []byte) error {
	err := os.WriteFile(filepath, data, 0o755)
	return err
}

func (t *Transcoder) getResolution(varient string) string {
	resolution, ok := t.VideoResolutionMap[varient]
	if !ok {
		resolution = t.VideoResolutionMap["720p"]
	}
	return resolution
}

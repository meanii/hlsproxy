package transcoder

import (
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/grafov/m3u8"
	"github.com/meanii/hlsproxy/config"
	"github.com/meanii/hlsproxy/internal/externalcmd"
	"github.com/meanii/hlsproxy/pkg/utils"
	"go.uber.org/zap"
)

const DefaultResolution = "720p"

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

func (vc VideoCodecType) StringFourCC() string {
	switch vc {
	case H265:
		return "hev1.1.6.L93.B0"
	case H264:
		return "avc1.4d40"
	}
	return "avc1.4d40"
}

func (ac AudioCodecType) StringFourCC() string {
	switch ac {
	case AAC:
		return "mp4a.40.5"
	}
	return "mp4a.40.5"
}

func (vc VideoCodecType) String() string {
	switch vc {
	case H265:
		return "H265"
	case H264:
		return "H264"
	}
	return "H264"
}

func (ac AudioCodecType) String() string {
	switch ac {
	case AAC:
		return "AAC"
	}
	return "AAC"
}

type Transcoder struct {
	ID             string
	MasterFileName string
	FfmpegBin      string
	Source         string
	Varients       []string
	VideoCodec     VideoCodecType
	AudioCodec     AudioCodecType
	FrameRate      float64
	AudioEnable    bool
	OutputDir      string
	MasterHls      string
	Mux            sync.RWMutex
	wg             sync.WaitGroup
}

func NewTranscoder(source string, ID string) *Transcoder {
	tscconfig := Transcoder{
		Source: source,
		ID:     ID,
	}

	tscconfig.FfmpegBin = config.GetConfig("").Config.Ffmpeg.Bin
	tscconfig.Varients = []string{"240p", "360p", "audio"}

	tscconfig.VideoCodec = H264
	tscconfig.AudioCodec = AAC

	tscconfig.MasterFileName = "playlist.m3u8"
	return &tscconfig
}

func (t *Transcoder) SetConfig(varients []string, audio bool, videoCodec string, audioCodec string) {
	if len(varients) >= 1 {
		t.Varients = varients
		zap.S().Infof("setting up varients %s", varients)
	}

	t.AudioEnable = audio
	if audio {
		t.Varients = append(t.Varients, "audio")
	}

	if videoCodec != "" {
		zap.S().Infof("setting up videoCodec %s", videoCodec)
		switch videoCodec {
		case "h264":
			t.VideoCodec = H264
		case "h265":
			t.VideoCodec = H265
		default:
			t.VideoCodec = H264
		}
	}

	if audioCodec != "" {
		zap.S().Infof("setting up audioCodec %s", audioCodec)
		switch audioCodec {
		case "AAC":
			t.AudioCodec = AAC
		default:
			t.AudioCodec = AAC
		}
	}
}

func (t *Transcoder) Run() (string, error) {
	t.prepareOutputDir()
	cmdstring := t.generateCmdString()
	_, err := t.generateMasterHls()
	if err != nil {
		zap.S().Errorf("failed to start trasncoder, Error: %s", err)
	}

	initialMasterHls := m3u8.NewMasterPlaylist()

	for index, varient := range t.Varients {
		varientName := t.Varients[index]
		varientURL := fmt.Sprintf("http://localhost:8001/hlsproxy/%s/%s/%s.m3u8", t.ID, varientName, varientName)
		genrateVarient := m3u8.Variant{
			URI:           varientURL,
			VariantParams: t.getVideoMeatadata(varient),
		}
		initialMasterHls.Variants = append(initialMasterHls.Variants, &genrateVarient)

		zap.S().Infof("transcoder, generated %s.m3u8 hls file, %s", varientName)
	}

	t.wg.Add(1)

	zap.S().Infof("transcoder, generated master.m3u8 hls file, %s\nm3u8file: %s", initialMasterHls.Variants[len(initialMasterHls.Variants)-1].URI, initialMasterHls.String())

	cmdrunnerpool := externalcmd.NewPool()
	rtmpPullCmd := externalcmd.NewCmd(
		cmdrunnerpool, cmdstring, true, make(externalcmd.Environment), nil)
	rtmpPullCmd.SetStreamID(t.ID)

	t.isReadToPlay()
	t.wg.Wait()
	return initialMasterHls.String(), nil
}

func (t *Transcoder) isReadToPlay() {
	zap.S().Infof("transcoder: waiting for transcoder to start")
	t.checkGeneratedTS()
	zap.S().Infof("transcoder: ready to play")
}

func (t *Transcoder) checkGeneratedTS() error {
	var foundTSChunks bool
	for {
		files := utils.FindFiles(t.OutputDir, ".ts")
		zap.S().Infof("transcoder: found %d chunks", len(files))
		for _, file := range files {
			zap.S().Infof("transcoder: found %s file", file)
			if strings.HasSuffix(file, ".ts") {
				zap.S().Infof("transcoder: found %s", file)
				foundTSChunks = true
				break
			}
		}
		if foundTSChunks {
			break
		}
		time.Sleep(1 * time.Second)
	}
	t.wg.Done()
	return nil
}

func (t *Transcoder) generateCmdString() string {
	prefix := fmt.Sprintf("%s -i \"%s\" -loglevel repeat+level+verbose -profile:v baseline -level 3.0 ", t.FfmpegBin, t.Source)
	suffixtree := make([]string, 0)

	for _, varient := range t.Varients {
		resolution := t.getVideoMeatadata(varient).Resolution
		if varient != "audio" {
			segmentFilename := "%03d.ts"
			cmd := fmt.Sprintf("-s %s -start_number 0 -hls_time 2 -hls_list_size 10 -hls_flags delete_segments+split_by_time -hls_segment_filename %s/%s/%s -b:v 500k -maxrate 500k -bufsize 1000k -f hls %s/%s/%s.m3u8 ",
				resolution,
				t.OutputDir,
				varient,
				segmentFilename,
				t.OutputDir,
				varient,
				varient,
			)
			suffixtree = append(suffixtree, cmd)
		}

		if varient == "audio" {
			segmentFilename := "%03d.ts"
			cmd := fmt.Sprintf("-map 0:a -start_number 0 -hls_time 2 -hls_list_size 10 -hls_flags delete_segments+split_by_time -hls_segment_filename %s/audio/%s -b:a 128k -f hls %s/audio/audio.m3u8 ",
				t.OutputDir,
				segmentFilename,
				t.OutputDir,
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
	t.OutputDir = path.Join(wd, config.GetConfig("").Config.Output.Dirname, t.ID)
	zap.S().Infof("setting up output dir: %s", t.OutputDir)

	for _, varient := range t.Varients {
		varientPath := path.Join(t.OutputDir, varient)
		err := os.MkdirAll(varientPath, os.ModePerm)
		if err != nil {
			zap.S().Warnf("transcoder: failed to mkdir, Error: %s", err)
		}
	}
}

func (t *Transcoder) generateMasterHls() (m3u8.MasterPlaylist, error) {
	masterHls := m3u8.NewMasterPlaylist()

	for _, varient := range t.Varients {
		genrateVarient := m3u8.Variant{
			URI:           fmt.Sprintf("%s/%s.m3u8", varient, varient),
			VariantParams: t.getVideoMeatadata(varient),
		}
		masterHls.Variants = append(masterHls.Variants, &genrateVarient)
	}

	zap.S().Infof("transcoder, generated master.m3u8 hls file, %s", masterHls.String())
	masterfilepath := path.Join(t.OutputDir, t.MasterFileName)
	t.MasterHls = masterfilepath
	err := t.writeFile(masterfilepath, []byte(masterHls.String()))
	return *masterHls, err
}

func (t *Transcoder) writeFile(filepath string, data []byte) error {
	err := os.WriteFile(filepath, data, 0o755)
	return err
}

func (t *Transcoder) getVideoMeatadata(varient string) m3u8.VariantParams {
	ONEKBPS := uint32(1000)

	videoVarients := map[string]m3u8.VariantParams{
		"240p": {
			ProgramId:  1,
			Resolution: "426x240",
			Name:       "240p",
			Codecs:     t.VideoCodec.StringFourCC(),
			Bandwidth:  500 * ONEKBPS,
			FrameRate:  t.FrameRate,
		},
		"360p": {
			ProgramId:  2,
			Resolution: "640x360",
			Name:       "360p",
			Codecs:     t.VideoCodec.StringFourCC(),
			Bandwidth:  1200 * ONEKBPS,
			FrameRate:  t.FrameRate,
		},
		"480p": {
			ProgramId:  3,
			Resolution: "854x480",
			Name:       "480p",
			Codecs:     t.VideoCodec.StringFourCC(),
			Bandwidth:  3000 * ONEKBPS,
			FrameRate:  t.FrameRate,
		},
		"720p": {
			ProgramId:  4,
			Resolution: "1280x720",
			Name:       "720p",
			Codecs:     t.VideoCodec.StringFourCC(),
			Bandwidth:  5000 * ONEKBPS,
			FrameRate:  t.FrameRate,
		},
		"1080p": {
			ProgramId:  5,
			Resolution: "192x1080",
			Name:       "1080p",
			Codecs:     t.VideoCodec.StringFourCC(),
			Bandwidth:  7000 * ONEKBPS,
			FrameRate:  t.FrameRate,
		},
		"1440p": {
			ProgramId:  6,
			Resolution: "2560x1440",
			Name:       "2K",
			Codecs:     t.VideoCodec.StringFourCC(),
			Bandwidth:  10000 * ONEKBPS,
			FrameRate:  t.FrameRate,
		},
		"2160p": {
			ProgramId:  7,
			Resolution: "3840x2160",
			Name:       "4K",
			Codecs:     t.VideoCodec.StringFourCC(),
			Bandwidth:  15000 * ONEKBPS,
			FrameRate:  t.FrameRate,
		},
		"audio": {
			ProgramId: 8,
			Name:      "audio",
			Codecs:    t.AudioCodec.StringFourCC(),
			Bandwidth: 192 * ONEKBPS,
		},
	}

	for _, varientmd := range videoVarients {
		if t.AudioEnable {
			audio, ok := videoVarients["audio"]
			if !ok {
				zap.S().Warn("didnt get any audio meatdata")
			}
			varientmd.Audio = audio.Name //nolint:noused
		}
	}

	metadata, found := videoVarients[varient]
	if !found {
		return videoVarients[DefaultResolution]
	}
	return metadata
}

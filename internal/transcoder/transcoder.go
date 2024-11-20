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
	Mux                sync.RWMutex
	wg                 sync.WaitGroup
	Ready              chan bool
}

func NewTranscoder(sourceHls string, ID string) *Transcoder {
	config := Transcoder{
		SourceHls: sourceHls,
		ID:        ID,
	}
	config.FfmpegBin = "/usr/bin/ffmpeg"
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
		varientUrl := fmt.Sprintf("http://localhost:8001/hlsproxy/%s/%s/%s.m3u8", t.ID, varientName, varientName)
		genrateVarient := m3u8.Variant{
			URI: varientUrl,
			VariantParams: m3u8.VariantParams{
				Resolution: t.getResolution(varient),
			},
		}
		initialMasterHls.Variants = append(initialMasterHls.Variants, &genrateVarient)

		zap.S().Infof("transcoder, generated %s.m3u8 hls file, %s", varientName)
	}

	t.wg.Add(1)

	zap.S().Infof("transcoder, generated master.m3u8 hls file, %s\nm3u8file: %s", initialMasterHls.Variants[len(initialMasterHls.Variants)-1].URI, initialMasterHls.String())

	cmdrunnerpool := externalcmd.NewPool()
	_ = externalcmd.NewCmd(
		cmdrunnerpool, cmdstring, true, make(externalcmd.Environment), nil)

	t.isReadToPlay()
	t.wg.Wait()
	return initialMasterHls.String(), nil
}

func (t *Transcoder) isReadToPlay() {
	zap.S().Infof("transcoder: waiting for transcoder to start")
	t.checkGeneratedTs()
	zap.S().Infof("transcoder: ready to play")
}

func (t *Transcoder) checkGeneratedTs() error {
	var foundTsChunks bool
	for {
		files := utils.FindFiles(t.CacheDir, ".ts")
		zap.S().Infof("transcoder: found %d chunks", len(files))
		for _, file := range files {
			zap.S().Infof("transcoder: found %s file", file)
			if strings.HasSuffix(file, ".ts") {
				zap.S().Infof("transcoder: found %s", file)
				foundTsChunks = true
				break
			}
		}
		if foundTsChunks {
			break
		}
		time.Sleep(1 * time.Second)
	}
	t.wg.Done()
	return nil
}

func (t *Transcoder) generateCmdString() string {
	prefix := fmt.Sprintf("%s -i \"%s\" -loglevel repeat+level+verbose -profile:v baseline -level 3.0 ", t.FfmpegBin, t.SourceHls)
	suffixtree := make([]string, 0)

	for _, varient := range t.Varients {
		resolution := t.getResolution(varient)
		if varient != "audio" {
			segmentFilename := "%03d.ts"
			cmd := fmt.Sprintf("-s %s -start_number 0 -hls_time 2 -hls_list_size 10 -hls_flags delete_segments+split_by_time -hls_segment_filename %s/%s/%s -b:v 500k -maxrate 500k -bufsize 1000k -f hls %s/%s/%s.m3u8 ",
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
			cmd := fmt.Sprintf("-map 0:a -start_number 0 -hls_time 2 -hls_list_size 10 -hls_flags delete_segments+split_by_time -hls_segment_filename %s/audio/%s -b:a 128k -f hls %s/audio/audio.m3u8 ",
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

func (t *Transcoder) generateMasterHls() (m3u8.MasterPlaylist, error) {
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
	return *masterHls, err
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

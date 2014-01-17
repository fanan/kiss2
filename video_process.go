package main

import (
    "bytes"
    "encoding/json"
    "errors"
    "fmt"
    "os/exec"
    "strings"
)

var (
    ErrFFMPEGNoInput error = errors.New("ffmpeg error: no input")
)

const (
    FF_FORMAT_MP4       string = "mov,mp4,m4a,3gp,3g2,mj2"
    FF_CODEC_TYPE_AUDIO string = "audio"
    FF_CODEC_TYPE_VIDEO string = "video"
    FF_CODEC_H264       string = "h264"
    FF_CODEC_AAC        string = "aac"
    FF_CODEC_COPY       string = "copy"
)

type ffStream struct {
    CodecName string `json:"codec_name"`
    CodecType string `json:"codec_type"`
}

type ffFormat struct {
    FormatName string `json:"format_name"`
    NbStreams  int    `json:"nb_streams"`
}

type FFInfo struct {
    Streams []ffStream `json:"streams"`
    Format  ffFormat   `json:"format"`
}

func detectVideoInfo(fn string) (videoReencode bool, audioReencode bool, formatIsMP4 bool, err error) {
    //cmd: ffprobe -v quiet -print_format json -show_format -show_streams filename
    stdout := new(bytes.Buffer)
    stderr := new(bytes.Buffer)
    cmd := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", fn)
    cmd.Stderr = stderr
    cmd.Stdout = stdout
    err = cmd.Run()
    if err != nil {
        err = errors.New(stderr.String() + err.Error())
        return false, false, false, err
    }
    ffinfo := new(FFInfo)
    err = json.Unmarshal(stdout.Bytes(), ffinfo)
    if err != nil {
        return false, false, false, err
    }
    formatIsMP4 = ffinfo.Format.FormatName == FF_FORMAT_MP4
    for _, stream := range ffinfo.Streams {
        switch stream.CodecType {
        case FF_CODEC_TYPE_AUDIO:
            audioReencode = stream.CodecName != FF_CODEC_AAC
        case FF_CODEC_TYPE_VIDEO:
            videoReencode = stream.CodecName != FF_CODEC_H264
        }
    }
    return videoReencode, audioReencode, formatIsMP4, nil
}

func combineTsToMp4(output string, inputs ...string) error {
    if len(inputs) == 0 {
        return ErrFFMPEGNoInput
    }
    var arg string
    if len(inputs) == 1 {
        arg = inputs[0]
    } else {
        arg = fmt.Sprintf("concat:%s", strings.Join(inputs, "|"))
    }
    cmd := exec.Command("ffmpeg", "-i", arg, "-c", "copy", "-bsf:a", "aac_adtstoasc", output)
    stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
    cmd.Stderr, cmd.Stdout = stderr, stdout
    err := cmd.Run()
    if err != nil {
        err = errors.New(stderr.String() + err.Error())
        return err
    }
    return nil
}

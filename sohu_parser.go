package main

import (
    "bufio"
    "encoding/json"
    "errors"
    "fmt"
    "io/ioutil"
    "net/http"
    "strconv"
    "strings"
)

const (
    sohu_pattern_vid        string = "var vid="
    sohu_video_parse_url1   string = "http://hot.vrs.sohu.com/vrs_flash.action?vid=%d"
    sohu_clip_url_prefix    string = "http://data.vod.itc.cn/"
    sohu_pattern_clip_parse string = "http://%s/?prot=2&t=0.6010&file=%s&new=%s"
    sohu_clip_url           string = "%s%s?key=%s"
)

type SohuVideoParser struct {
    owner *Video
    vid   int
    info  *sohuInfo
}

func (self *SohuVideoParser) SetOwner(video *Video) {
    self.owner = video
}

func (self *SohuVideoParser) GetOwner() *Video {
    return self.owner
}

type sohuClipInfo struct {
    Su    string
    Url   string
    Allot string
}

func (self *SohuVideoParser) Parse() error {
    var err error
    for i := 0; i < Lives; i++ {
        err = self.getVid()
        if err != nil {
            continue
        }
        err = self.parse()
        if err != nil {
            continue
        }
        if self.choose() {
            err = self.parse()
            if err != nil {
                continue
            }
        }
        self.owner.nClips = self.info.Data.Numbers
        self.owner.clips = make([]*Clip, 0, self.owner.nClips)
        self.owner.Name = self.info.Data.Name
        self.info.Url = self.info.Url
        for i := 0; i < self.owner.nClips; i++ {
            clip := new(Clip)
            clip.parser = new(SohuClipParser)
            clip.parser.SetOwner(clip)
            clip.logger = self.owner.logger
            clip.total = self.info.Data.ClipsBytes[i]
            info := sohuClipInfo{Su: self.info.Data.Su[i], Url: strings.TrimPrefix(self.info.Data.ClipsUrl[i], sohu_clip_url_prefix), Allot: self.info.Allot}
            clip.SetInfo(info)
            self.owner.clips = append(self.owner.clips, clip)
        }
        return nil
    }
    return ErrParseBadFormat
}

func (self *SohuVideoParser) getVid() error {
    resp, err := http.Get(self.owner.Url)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    br := bufio.NewReader(resp.Body)
    for {
        line, err := br.ReadString('\n')
        if err != nil {
            return err
        }
        line = strings.TrimSpace(line)
        if strings.HasPrefix(line, sohu_pattern_vid) {
            items := strings.Split(line, "\"")
            if len(items) > 2 {
                self.vid, err = strconv.Atoi(items[1])
                if err != nil {
                    return err
                }
                return nil
            }
        }
    }
    return errors.New("Cannot get vid")
}

type sohuInfo struct {
    Url   string       `json:url`
    Vid   int          `json:"vid"`
    Allot string       `json:"allot"`
    Data  sohuInfoData `json:"data"`
}

type sohuInfoData struct {
    ClipsUrl   []string `json:"clipsURL"`
    ClipsBytes []int64  `json:"clipsBytes"`
    Su         []string `json:"su"`
    Name       string   `json:"tvName"`
    Fps        int      `json:"fps"`
    NorVid     int      `json:"norVid"`
    HighVid    int      `json:"highVid"`
    SuperVid   int      `json:"superVid"`
    OriVid     int      `json:"oriVid"`
    Numbers    int      `json:"totalBlocks"`
}

func (self *SohuVideoParser) parse() error {
    resp, err := http.Get(fmt.Sprintf(sohu_video_parse_url1, self.vid))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    data, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return err
    }
    self.info = new(sohuInfo)
    err = json.Unmarshal(data, self.info)
    if err != nil {
        return err
    }
    if self.info.Data.Numbers != len(self.info.Data.ClipsBytes) || self.info.Data.Numbers != len(self.info.Data.ClipsUrl) || self.info.Data.Numbers != len(self.info.Data.Su) {
        return ErrParseBadFormat
    }
    return nil
}

func (self *SohuVideoParser) choose() bool {
    if self.info.Data.OriVid != 0 && self.info.Data.OriVid != self.vid {
        self.vid = self.info.Data.OriVid
        return true
    }
    if self.info.Data.SuperVid != 0 && self.info.Data.SuperVid != self.vid {
        self.vid = self.info.Data.SuperVid
        return true
    }
    if self.info.Data.HighVid != 0 && self.info.Data.HighVid != self.vid {
        self.vid = self.info.Data.HighVid
        return true
    }
    if self.info.Data.NorVid != 0 && self.info.Data.NorVid != self.vid {
        self.vid = self.info.Data.NorVid
        return true
    }
    return false
}

type SohuClipParser struct {
    owner *Clip
}

func (self *SohuClipParser) SetOwner(c *Clip) {
    self.owner = c
}

func (self *SohuClipParser) GetOwner() *Clip {
    return self.owner
}

func (self *SohuClipParser) parse() (url string, err error) {
    info, ok := self.owner.info.(sohuClipInfo)
    if !ok {
        return url, ErrParseBadFormat
    }
    resp, err := http.Get(fmt.Sprintf(sohu_pattern_clip_parse, info.Allot, info.Url, info.Su))
    if err != nil {
        return url, err
    }
    defer resp.Body.Close()
    data, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return url, err
    }
    s := string(data)
    if !strings.HasPrefix(s, "http://") {
        return url, ErrProtcolUnsupported
    }
    items := strings.Split(s, "|")
    if len(items) < 4 {
        return url, ErrParseBadFormat
    }
    url = fmt.Sprintf(sohu_clip_url, items[0], info.Su, items[3])
    return url, nil
}

func (self *SohuClipParser) Parse() (url string, err error) {
    for i := 0; i < Lives; i++ {
        url, err = self.parse()
        if err != nil {
            continue
        }
        return url, nil
    }
    return url, err
}
